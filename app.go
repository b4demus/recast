package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Settings struct {
	Lang        string            `json:"lang"`
	Theme       string            `json:"theme"`  // system | light | dark
	Accent      string            `json:"accent"` // app.js ACCENTS key
	Kind        string            `json:"kind"`   // open tab of the format picker
	Format      string            `json:"format"`
	Codecs      map[string]string `json:"codecs"` // chosen codec per format
	Quality     string            `json:"quality"` // high | mid | low
	Res         int               `json:"res"`     // frame height cap, 0 = original
	HW          bool              `json:"hw"`
	OutMode     string            `json:"outMode"` // same | dir
	OutDir      string            `json:"outDir"`
	Parallel    int               `json:"parallel"`
	Overwrite   bool              `json:"overwrite"`
	LowPriority bool              `json:"lowPriority"`
	CustomExt   string            `json:"customExt"`
	CustomArgs  string            `json:"customArgs"`
	FFmpeg      string            `json:"ffmpeg"` // picked ffmpeg.exe, "" = find it
}

func defaultSettings() Settings {
	return Settings{
		Lang: "ru", Theme: "system", Accent: "clay", Kind: "video", Format: "mp4",
		Codecs: map[string]string{}, Quality: "mid", OutMode: "same", Parallel: 1, LowPriority: true,
	}
}

// codec is the chosen codec for d, or its default.
func (s Settings) codec(d *formatDef) string {
	if c := s.Codecs[d.id]; c != "" {
		for _, x := range d.codecs {
			if x == c {
				return c
			}
		}
	}
	if len(d.codecs) > 0 {
		return d.codecs[0]
	}
	return ""
}

type windowSize struct {
	W int `json:"w"`
	H int `json:"h"`
}

type stateFile struct {
	Version  int        `json:"version"`
	Settings Settings   `json:"settings"`
	Window   windowSize `json:"window"`
}

// Job is one file in the queue as the UI sees it.
type Job struct {
	ID       string  `json:"id"`
	Path     string  `json:"path"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Kind     string  `json:"kind"`
	Summary  string  `json:"summary"`
	Duration float64 `json:"duration"`
	Status   string  `json:"status"`   // probing | ready | running | done | error | canceled
	Progress float64 `json:"progress"` // 0..1, -1 = unknown
	Speed    float64 `json:"speed"`
	ETA      float64 `json:"eta"`
	Out      string  `json:"out"`
	OutSize  int64   `json:"outSize"`
	Error    string  `json:"error"`
	Took     float64 `json:"took"`
}

type job struct {
	Job
	info     *probeInfo
	cmd      *exec.Cmd
	log      ring
	cmdline  string
	canceled bool // the user stopped this one
	requeue  bool // stopped with the whole queue: goes back to ready
}

type State struct {
	Settings Settings `json:"settings"`
	Jobs     []Job    `json:"jobs"`
	Running  bool     `json:"running"`
	Tools    *tools   `json:"ffmpeg"` // nil while it is being looked at
	Formats  []Format `json:"formats"`
	Version  string   `json:"version"`
}

type App struct {
	ctx      context.Context
	dataDir  string
	mu       sync.Mutex
	data     stateFile
	tools    *tools
	ready    chan struct{} // closed once tools are first loaded
	jobs     []*job
	running  bool
	runStats [2]int // done, failed since Start
	reserved map[string]bool
	nextID   int
	dirty    atomic.Bool
	probeSem chan struct{}
	kill     *killJob
	title    string
}

func NewApp() *App {
	a := &App{
		ready:    make(chan struct{}),
		reserved: map[string]bool{},
		probeSem: make(chan struct{}, 4),
		kill:     newKillJob(),
	}
	a.dataDir = pickDataDir()
	a.data = stateFile{Version: 1, Settings: defaultSettings()}
	if b, err := os.ReadFile(filepath.Join(a.dataDir, "state.json")); err == nil {
		_ = json.Unmarshal(b, &a.data)
	}
	a.data.Settings = sanitize(a.data.Settings)
	return a
}

// pickDataDir keeps everything next to the exe when that folder is writable,
// so recast stays portable; otherwise %APPDATA%\recast.
func pickDataDir() string {
	if exe, err := os.Executable(); err == nil {
		d := filepath.Join(filepath.Dir(exe), "data")
		if os.MkdirAll(d, 0o755) == nil {
			probe := filepath.Join(d, ".w")
			if os.WriteFile(probe, nil, 0o644) == nil {
				os.Remove(probe)
				return d
			}
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	d := filepath.Join(base, "recast")
	os.MkdirAll(d, 0o755)
	return d
}

func sanitize(s Settings) Settings {
	def := defaultSettings()
	if s.Lang != "en" {
		s.Lang = "ru"
	}
	if s.Theme != "light" && s.Theme != "dark" {
		s.Theme = "system"
	}
	if s.Accent == "" {
		s.Accent = def.Accent
	}
	d := findFormat(s.Format)
	if d == nil {
		s.Format, d = def.Format, findFormat(def.Format)
	}
	if s.Kind != "video" && s.Kind != "audio" && s.Kind != "image" {
		s.Kind = d.kind
	}
	if s.Codecs == nil {
		s.Codecs = map[string]string{}
	}
	if s.Quality != "high" && s.Quality != "low" {
		s.Quality = "mid"
	}
	switch s.Res {
	case 0, 2160, 1440, 1080, 720, 480:
	default:
		s.Res = 0
	}
	if s.OutMode != "dir" {
		s.OutMode = "same"
	}
	if s.Parallel < 1 || s.Parallel > 4 {
		s.Parallel = 1
	}
	s.CustomExt = strings.Trim(strings.TrimSpace(s.CustomExt), ".")
	if strings.ContainsAny(s.CustomExt, `\/:*?"<>| `) {
		s.CustomExt = ""
	}
	return s
}

func (a *App) saveLocked() error {
	b, err := json.MarshalIndent(a.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(a.dataDir, "state.json.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(a.dataDir, "state.json"))
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go func() {
		t := loadTools(a.data.Settings.FFmpeg)
		a.mu.Lock()
		a.tools = t
		a.mu.Unlock()
		close(a.ready)
		a.touch()
	}()
	go a.emitLoop()
	// files passed on the command line ("Send to", drag onto the exe)
	a.AddPaths(os.Args[1:])
}

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = false
	for _, j := range a.jobs {
		if j.cmd != nil && j.cmd.Process != nil {
			j.canceled = true
			j.cmd.Process.Kill()
		}
	}
}

func (a *App) secondInstance(d options.SecondInstanceData) {
	a.AddPaths(d.Args)
	wruntime.WindowUnminimise(a.ctx)
	wruntime.Show(a.ctx)
}

func (a *App) touch() { a.dirty.Store(true) }

// emitLoop batches state pushes: progress lines arrive far faster than the UI needs.
func (a *App) emitLoop() {
	tick := time.NewTicker(120 * time.Millisecond)
	defer tick.Stop()
	for range tick.C {
		if !a.dirty.Swap(false) {
			continue
		}
		st := a.GetState()
		wruntime.EventsEmit(a.ctx, "state", st)
		a.setTitle(st)
	}
}

// setTitle shows the queue's progress on the taskbar button.
func (a *App) setTitle(st State) {
	title := "recast"
	if st.Running {
		var sum float64
		n := 0
		for _, j := range st.Jobs {
			switch j.Status {
			case "running":
				sum += max(j.Progress, 0)
				n++
			case "ready", "probing":
				n++
			}
		}
		if n > 0 {
			title = fmt.Sprintf("%d%% · recast", int(sum/float64(n)*100))
		}
	}
	if title != a.title {
		a.title = title
		wruntime.WindowSetTitle(a.ctx, title)
	}
}

// ---- bound methods ----

func (a *App) GetState() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := State{Settings: a.data.Settings, Running: a.running, Version: version, Jobs: make([]Job, len(a.jobs))}
	for i, j := range a.jobs {
		st.Jobs[i] = j.Job
	}
	if a.tools != nil {
		t := *a.tools
		st.Tools = &t
		st.Formats = a.tools.formats()
	}
	return st
}

var mediaExt = map[string]bool{}

func init() {
	for _, e := range strings.Fields(`mp4 mkv mov avi webm m4v wmv flv ts m2ts mts mpg mpeg vob ogv 3gp 3g2 mxf f4v asf divx rm rmvb gif
		mp3 m4a m4b aac wav flac ogg oga opus wma aiff aif aifc ac3 eac3 dts ape wv mka amr caf mp2 au tta
		png jpg jpeg jfif webp avif bmp tif tiff ico heic heif jxl tga exr dds psd`) {
		mediaExt["."+e] = true
	}
}

// AddFiles opens the system file picker.
func (a *App) AddFiles() int {
	var pats []string
	for e := range mediaExt {
		pats = append(pats, "*"+e)
	}
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Filters: []wruntime.FileFilter{
			{DisplayName: "Media", Pattern: strings.Join(pats, ";")},
			{DisplayName: "*.*", Pattern: "*.*"},
		},
	})
	if err != nil {
		return 0
	}
	return a.AddPaths(paths)
}

// AddPaths queues files; folders are walked for known media extensions.
// Files given one by one are taken whatever the extension: ffprobe decides.
func (a *App) AddPaths(paths []string) int {
	var files []string
	for _, p := range paths {
		if strings.HasPrefix(p, "-") {
			continue
		}
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			files = append(files, p)
			continue
		}
		filepath.WalkDir(p, func(f string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && mediaExt[strings.ToLower(filepath.Ext(f))] {
				files = append(files, f)
			}
			return nil
		})
	}
	a.mu.Lock()
	queued := map[string]bool{}
	for _, j := range a.jobs {
		if j.Status == "probing" || j.Status == "ready" || j.Status == "running" {
			queued[strings.ToLower(j.Path)] = true
		}
	}
	var added []*job
	for _, f := range files {
		if abs, err := filepath.Abs(f); err == nil {
			f = abs
		}
		if queued[strings.ToLower(f)] {
			continue
		}
		queued[strings.ToLower(f)] = true
		a.nextID++
		j := &job{Job: Job{ID: strconv.Itoa(a.nextID), Path: f, Name: filepath.Base(f), Status: "probing", Progress: 0}}
		if st, err := os.Stat(f); err == nil {
			j.Size = st.Size()
		}
		a.jobs = append(a.jobs, j)
		added = append(added, j)
	}
	a.mu.Unlock()
	for _, j := range added {
		go a.probeJob(j)
	}
	if len(added) > 0 {
		a.touch()
	}
	return len(added)
}

func (a *App) probeJob(j *job) {
	<-a.ready
	a.probeSem <- struct{}{}
	defer func() { <-a.probeSem }()
	a.mu.Lock()
	t := a.tools
	a.mu.Unlock()
	var info *probeInfo
	err := errors.New("e.noFFmpeg")
	if t.ok() {
		info, err = t.probe(j.Path)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if j.Status != "probing" { // removed meanwhile
		return
	}
	if err != nil {
		j.Status, j.Error = "error", err.Error()
	} else {
		j.info, j.Status = info, "ready"
		j.Kind, j.Summary, j.Duration = info.Kind, info.summary(), info.Duration
	}
	a.pumpLocked()
	a.touch()
}

func (a *App) find(id string) *job {
	for _, j := range a.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// Remove drops a file from the queue, stopping it first when it is running.
func (a *App) Remove(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, j := range a.jobs {
		if j.ID != id {
			continue
		}
		if j.Status == "running" {
			a.cancelLocked(j, false)
		}
		j.Status = "removed"
		a.jobs = append(a.jobs[:i], a.jobs[i+1:]...)
		break
	}
	a.pumpLocked()
	a.touch()
}

// ClearFinished drops everything that is not waiting or running.
func (a *App) ClearFinished() {
	a.mu.Lock()
	defer a.mu.Unlock()
	kept := a.jobs[:0]
	for _, j := range a.jobs {
		if j.Status == "ready" || j.Status == "running" || j.Status == "probing" {
			kept = append(kept, j)
		}
	}
	a.jobs = kept
	a.touch()
}

// ClearAll empties the queue except for what is running right now.
func (a *App) ClearAll() {
	a.mu.Lock()
	defer a.mu.Unlock()
	kept := a.jobs[:0]
	for _, j := range a.jobs {
		if j.Status == "running" {
			kept = append(kept, j)
		} else {
			j.Status = "removed"
		}
	}
	a.jobs = kept
	a.touch()
}

// Start runs the queue. With nothing waiting it converts the finished files
// again: change the format, press the button once more.
func (a *App) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	waiting := false
	for _, j := range a.jobs {
		waiting = waiting || j.Status == "ready" || j.Status == "probing"
	}
	if !waiting {
		for _, j := range a.jobs {
			if (j.Status == "done" || j.Status == "error" || j.Status == "canceled") && j.info != nil {
				a.resetLocked(j)
			}
		}
	}
	a.running, a.runStats = true, [2]int{}
	a.pumpLocked()
	a.touch()
}

// Stop halts the queue; what was running goes back to waiting.
func (a *App) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = false
	for _, j := range a.jobs {
		if j.Status == "running" {
			a.cancelLocked(j, true)
		}
	}
	a.touch()
}

// Cancel stops one running file.
func (a *App) Cancel(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if j := a.find(id); j != nil && j.Status == "running" {
		a.cancelLocked(j, false)
	}
}

func (a *App) cancelLocked(j *job, requeue bool) {
	j.canceled, j.requeue = true, requeue
	if j.cmd != nil && j.cmd.Process != nil {
		j.cmd.Process.Kill()
	}
}

// Retry puts a finished file back in line and runs the queue.
func (a *App) Retry(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	j := a.find(id)
	if j == nil || j.info == nil || j.Status == "running" || j.Status == "probing" {
		return
	}
	a.resetLocked(j)
	if !a.running {
		a.running, a.runStats = true, [2]int{}
	}
	a.pumpLocked()
	a.touch()
}

func (a *App) resetLocked(j *job) {
	j.Status, j.Progress, j.Speed, j.ETA, j.Error, j.Out, j.OutSize, j.Took = "ready", 0, 0, 0, "", "", 0, 0
	j.canceled, j.requeue = false, false
}

// pumpLocked starts waiting files up to the parallel limit and notices the end of a run.
func (a *App) pumpLocked() {
	if !a.running || !a.tools.ok() {
		return
	}
	active, probing := 0, false
	for _, j := range a.jobs {
		switch j.Status {
		case "running":
			active++
		case "probing":
			probing = true
		}
	}
	s := a.data.Settings
	for _, j := range a.jobs {
		if active >= s.Parallel {
			break
		}
		if j.Status == "ready" {
			j.Status, j.Progress = "running", 0
			active++
			go a.run(j, s)
		}
	}
	if active == 0 && !probing {
		a.running = false
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "finished", map[string]int{"done": a.runStats[0], "failed": a.runStats[1]})
		}
	}
}

func (a *App) outPathLocked(in, ext string, s Settings) string {
	dir := filepath.Dir(in)
	if s.OutMode == "dir" && s.OutDir != "" {
		dir = s.OutDir
	}
	stem := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
	for n := 0; ; n++ {
		name := stem + "." + ext
		if n > 0 {
			name = fmt.Sprintf("%s (%d).%s", stem, n, ext)
		}
		p := filepath.Join(dir, name)
		if strings.EqualFold(p, in) || a.reserved[strings.ToLower(p)] {
			continue
		}
		if s.Overwrite {
			return p
		}
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			return p
		}
	}
}

func (a *App) run(j *job, s Settings) {
	d := findFormat(s.Format)
	codec := s.codec(d)
	ext := d.ext
	if s.CustomExt != "" {
		ext = s.CustomExt
	}
	a.mu.Lock()
	t := a.tools
	out := a.outPathLocked(j.Path, ext, s)
	key := strings.ToLower(out)
	a.reserved[key] = true
	j.Out, j.log = out, ring{}
	if d.kind == "image" && s.CustomExt == "" || j.Duration <= 0 {
		j.Progress = -1
	}
	a.mu.Unlock()
	a.touch()

	start := time.Now()
	args, err := buildArgs(t, s, d, codec, j.info, j.Path, out)
	if err == nil {
		cmd := command(t.FFmpeg, args...)
		if s.LowPriority {
			cmd.SysProcAttr.CreationFlags |= belowNormalPrio
		}
		a.mu.Lock()
		j.cmdline = quoteArgs(t.FFmpeg, args)
		a.mu.Unlock()
		os.MkdirAll(filepath.Dir(out), 0o755)
		err = encode(cmd, &j.log, func() {
			a.kill.add(cmd.Process)
			a.mu.Lock()
			j.cmd = cmd
			if j.canceled {
				cmd.Process.Kill()
			}
			a.mu.Unlock()
		}, func(p progress) {
			a.mu.Lock()
			if j.Duration > 0 && j.Progress >= 0 {
				j.Progress = min(p.outTime/j.Duration, 0.999)
				if p.speed > 0 {
					j.ETA = max(j.Duration-p.outTime, 0) / p.speed
				}
			}
			j.Speed, j.OutSize = p.speed, p.size
			a.mu.Unlock()
			a.touch()
		})
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.reserved, key)
	j.cmd = nil
	j.Took = time.Since(start).Seconds()
	switch {
	case j.Status == "removed":
		os.Remove(out)
	case j.requeue:
		os.Remove(out)
		a.resetLocked(j)
	case j.canceled:
		os.Remove(out)
		j.Status, j.Error = "canceled", ""
	case err != nil:
		os.Remove(out)
		msg := j.log.last()
		if strings.HasPrefix(err.Error(), "e.") || msg == "" {
			msg = err.Error()
		}
		j.Status, j.Error = "error", msg
		a.runStats[1]++
	default:
		j.Status, j.Progress, j.ETA = "done", 1, 0
		if st, err := os.Stat(out); err == nil {
			j.OutSize = st.Size()
		}
		a.runStats[0]++
	}
	a.pumpLocked()
	a.touch()
}

type JobLog struct {
	Cmd   string   `json:"cmd"`
	Lines []string `json:"lines"`
}

func (a *App) GetJobLog(id string) JobLog {
	a.mu.Lock()
	defer a.mu.Unlock()
	j := a.find(id)
	if j == nil {
		return JobLog{}
	}
	return JobLog{Cmd: j.cmdline, Lines: j.log.all()}
}

func (a *App) SaveSettings(s Settings) error {
	s = sanitize(s)
	a.mu.Lock()
	reload := s.FFmpeg != a.data.Settings.FFmpeg
	a.data.Settings = s
	err := a.saveLocked()
	a.mu.Unlock()
	if reload {
		a.reloadTools()
	}
	a.touch()
	return err
}

func (a *App) reloadTools() {
	<-a.ready
	a.mu.Lock()
	custom := a.data.Settings.FFmpeg
	a.tools = nil
	a.mu.Unlock()
	a.touch()
	t := loadTools(custom)
	a.mu.Lock()
	a.tools = t
	// files that failed only for want of ffmpeg get another look
	var again []*job
	for _, j := range a.jobs {
		if j.Status == "error" && j.Error == "e.noFFmpeg" {
			j.Status, j.Error = "probing", ""
			again = append(again, j)
		}
	}
	a.mu.Unlock()
	for _, j := range again {
		go a.probeJob(j)
	}
	a.touch()
}

func (a *App) PickOutDir() string {
	a.mu.Lock()
	cur := a.data.Settings.OutDir
	a.mu.Unlock()
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{DefaultDirectory: cur})
	if err != nil {
		return ""
	}
	return dir
}

func (a *App) PickFFmpeg() string {
	p, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Filters: []wruntime.FileFilter{{DisplayName: "ffmpeg.exe", Pattern: "ffmpeg.exe"}},
	})
	if err != nil {
		return ""
	}
	return p
}

// Reveal shows a file selected in Explorer.
func (a *App) Reveal(path string) {
	cmd := exec.Command("explorer.exe")
	// explorer parses its own command line: /select,"path" must stay one piece
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	cmd.Start()
}

func (a *App) OpenPath(path string) {
	exec.Command("explorer.exe", path).Start()
}

func (a *App) Minimise() { wruntime.WindowMinimise(a.ctx) }

func (a *App) ToggleMaximise() { wruntime.WindowToggleMaximise(a.ctx) }

func (a *App) Quit() { wruntime.Quit(a.ctx) }

func (a *App) SetWindowDark(dark bool) {
	if dark {
		wruntime.WindowSetBackgroundColour(a.ctx, 20, 20, 20, 255)
	} else {
		wruntime.WindowSetBackgroundColour(a.ctx, 232, 230, 223, 255)
	}
}

func (a *App) SaveWindowSize(w, h int) {
	if w < minW || h < minH || w > 10000 || h > 10000 || wruntime.WindowIsMaximised(a.ctx) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.data.Window.W == w && a.data.Window.H == h {
		return
	}
	a.data.Window = windowSize{W: w, H: h}
	_ = a.saveLocked()
}
