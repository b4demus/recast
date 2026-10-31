package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	createNoWindow    = 0x08000000
	belowNormalPrio   = 0x00004000
	hwProbeTimeout    = 10 * time.Second
	probeTimeout      = 30 * time.Second
	logLinesKept      = 120
	progressMinPeriod = 100 * time.Millisecond
)

// tools is the ffmpeg build in use and what it can encode on this machine.
type tools struct {
	FFmpeg  string `json:"path"`
	FFprobe string `json:"-"`
	Version string `json:"version"`
	HWName  string `json:"hw"` // NVENC / QSV / AMF, "" when no GPU encoder works
	enc     map[string]bool
	hw      map[string]string // codec id -> working GPU encoder
}

func (t *tools) ok() bool          { return t != nil && t.FFmpeg != "" }
func (t *tools) has(e string) bool { return t.enc[e] }

func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// findTools looks for ffmpeg.exe with ffprobe.exe beside it: the user's pick,
// then next to recast.exe, then PATH.
func findTools(custom string) (ffmpeg, ffprobe string) {
	var cands []string
	if custom != "" {
		cands = append(cands, custom)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		cands = append(cands, filepath.Join(d, "ffmpeg.exe"), filepath.Join(d, "bin", "ffmpeg.exe"), filepath.Join(d, "ffmpeg", "bin", "ffmpeg.exe"))
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		cands = append(cands, p)
	}
	for _, c := range cands {
		probe := filepath.Join(filepath.Dir(c), "ffprobe.exe")
		if fileExists(c) && fileExists(probe) {
			return c, probe
		}
	}
	return "", ""
}

func loadTools(custom string) *tools {
	t := &tools{enc: map[string]bool{}, hw: map[string]string{}}
	t.FFmpeg, t.FFprobe = findTools(custom)
	if t.FFmpeg == "" {
		return t
	}
	if out, err := command(t.FFmpeg, "-hide_banner", "-version").Output(); err == nil {
		if f := strings.Fields(string(out)); len(f) > 2 {
			t.Version = strings.SplitN(f[2], "-", 2)[0]
		}
	}
	if out, err := command(t.FFmpeg, "-hide_banner", "-encoders").Output(); err == nil {
		past := false
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) == 1 && strings.HasPrefix(f[0], "---") {
				past = true
				continue
			}
			if past && len(f) >= 2 {
				t.enc[f[1]] = true
			}
		}
	}
	t.probeHW()
	return t
}

// probeHW tries every listed GPU encoder on a tiny clip: builds list NVENC,
// QSV and AMF whether or not the hardware is there.
func (t *tools) probeHW() {
	var mu sync.Mutex
	var wg sync.WaitGroup
	works := map[string]bool{}
	for _, list := range hwEncoders {
		for _, enc := range list {
			if !t.enc[enc] {
				continue
			}
			wg.Add(1)
			go func(enc string) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), hwProbeTimeout)
				defer cancel()
				cmd := command(t.FFmpeg, "-hide_banner", "-v", "error", "-f", "lavfi", "-i", "color=black:s=256x256:d=0.2",
					"-frames:v", "3", "-c:v", enc, "-f", "null", "-")
				if err := runCtx(ctx, cmd); err == nil {
					mu.Lock()
					works[enc] = true
					mu.Unlock()
				}
			}(enc)
		}
	}
	wg.Wait()
	names := map[string]string{"nvenc": "NVENC", "qsv": "QSV", "amf": "AMF"}
	for codec, list := range hwEncoders {
		for _, enc := range list {
			if works[enc] {
				t.hw[codec] = enc
				if t.HWName == "" || codec == "h264" {
					t.HWName = names[enc[strings.LastIndex(enc, "_")+1:]]
				}
				break
			}
		}
	}
}

func runCtx(ctx context.Context, cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		cmd.Process.Kill()
		<-done
		return ctx.Err()
	}
}

// probeInfo is what the queue needs to know about an input file.
type probeInfo struct {
	Kind     string // video | audio | image
	Duration float64
	HasVideo bool
	HasAudio bool
	W, H     int
	VCodec   string
	ACodec   string
	Rate     int
	Channels int
}

func (t *tools) probe(path string) (*probeInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := command(t.FFprobe, "-v", "error", "-print_format", "json", "-show_streams", "-show_format", path)
	var out strings.Builder
	cmd.Stdout = &out
	if err := runCtx(ctx, cmd); err != nil {
		return nil, fmt.Errorf("e.notMedia")
	}
	var p struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			SampleRate  string `json:"sample_rate"`
			Channels    int    `json:"channels"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal([]byte(out.String()), &p); err != nil {
		return nil, fmt.Errorf("e.notMedia")
	}
	info := &probeInfo{}
	info.Duration, _ = strconv.ParseFloat(p.Format.Duration, 64)
	for _, s := range p.Streams {
		switch s.CodecType {
		case "video":
			// cover art in music files is a picture, not a video track;
			// "ansi" is ffmpeg reading a text file as a terminal recording
			if s.Disposition.AttachedPic == 1 || s.CodecName == "ansi" || info.HasVideo {
				continue
			}
			info.HasVideo, info.VCodec, info.W, info.H = true, s.CodecName, s.Width, s.Height
		case "audio":
			if !info.HasAudio {
				info.HasAudio, info.ACodec, info.Channels = true, s.CodecName, s.Channels
				info.Rate, _ = strconv.Atoi(s.SampleRate)
			}
		}
	}
	fn := p.Format.FormatName
	switch {
	case info.HasVideo && (strings.HasSuffix(fn, "_pipe") || fn == "image2" || fn == "ico"):
		info.Kind, info.Duration = "image", 0
	case info.HasVideo:
		info.Kind = "video"
	case info.HasAudio:
		info.Kind = "audio"
	default:
		return nil, fmt.Errorf("e.notMedia")
	}
	return info, nil
}

// summary is the one-line description under a file name.
func (p *probeInfo) summary() string {
	var parts []string
	switch p.Kind {
	case "image":
		parts = append(parts, fmt.Sprintf("%d×%d", p.W, p.H), p.VCodec)
	case "video":
		parts = append(parts, fmt.Sprintf("%d×%d", p.W, p.H), p.VCodec)
		if p.HasAudio {
			parts = append(parts, p.ACodec)
		}
	case "audio":
		parts = append(parts, p.ACodec)
		if p.Rate > 0 {
			parts = append(parts, strconv.FormatFloat(float64(p.Rate)/1000, 'f', -1, 64)+" kHz")
		}
	}
	if p.Duration > 0 {
		parts = append(parts, clock(p.Duration))
	}
	return strings.Join(parts, " · ")
}

func clock(sec float64) string {
	s := int(sec + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// ring keeps the last lines of ffmpeg's stderr for the job details.
type ring struct {
	mu    sync.Mutex
	lines []string
}

func (r *ring) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, s)
	if len(r.lines) > logLinesKept {
		r.lines = r.lines[len(r.lines)-logLinesKept:]
	}
}

func (r *ring) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lines...)
}

func (r *ring) last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.lines) - 1; i >= 0; i-- {
		// the closing "Conversion failed!" says less than the line before it
		if s := strings.TrimSpace(r.lines[i]); s != "" && s != "Conversion failed!" {
			return s
		}
	}
	return ""
}

type progress struct {
	outTime float64 // seconds of output written
	speed   float64
	size    int64
}

// encode runs ffmpeg and calls onProgress with each -progress block.
func encode(cmd *exec.Cmd, log *ring, onStart func(), onProgress func(progress)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	onStart()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		readLines(stderr, func(s string) { log.add(s) })
	}()
	var cur progress
	last := time.Time{}
	readLines(stdout, func(s string) {
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			return
		}
		switch k {
		case "out_time_us":
			if us, err := strconv.ParseInt(v, 10, 64); err == nil && us >= 0 {
				cur.outTime = float64(us) / 1e6
			}
		case "total_size":
			cur.size, _ = strconv.ParseInt(v, 10, 64)
		case "speed":
			cur.speed, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), "x"), 64)
		case "progress":
			if time.Since(last) >= progressMinPeriod || v == "end" {
				last = time.Now()
				onProgress(cur)
			}
		}
	})
	wg.Wait()
	return cmd.Wait()
}

func readLines(r io.Reader, fn func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	// ffmpeg ends some status lines with \r
	sc.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		for i, b := range data {
			if b == '\n' || b == '\r' {
				return i + 1, data[:i], nil
			}
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	for sc.Scan() {
		if s := sc.Text(); s != "" {
			fn(s)
		}
	}
}

// quoteArgs renders a command line for the details panel, copy-paste ready.
func quoteArgs(exe string, args []string) string {
	parts := []string{quoteArg(filepath.Base(exe))}
	for _, a := range args {
		parts = append(parts, quoteArg(a))
	}
	return strings.Join(parts, " ")
}

func quoteArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"&|<>^;[]") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
