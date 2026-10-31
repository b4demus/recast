package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEveryFormat converts small generated clips into every format and codec
// this ffmpeg can serve and checks that ffprobe can read the result.
func TestEveryFormat(t *testing.T) {
	tl := loadTools("")
	if !tl.ok() {
		t.Skip("ffmpeg not found")
	}
	t.Logf("ffmpeg %s, gpu %q, hw %v", tl.Version, tl.HWName, tl.hw)
	dir := t.TempDir()
	src := map[string]string{
		"video": filepath.Join(dir, "src.mkv"),
		"audio": filepath.Join(dir, "src.flac"),
		"image": filepath.Join(dir, "src.png"),
	}
	gen := map[string][]string{
		// odd frame size on purpose: 4:2:0 encoders must still accept it
		"video": {"-f", "lavfi", "-i", "testsrc2=s=321x241:r=25:d=2", "-f", "lavfi", "-i", "sine=f=440:d=2", "-c:v", "libx264", "-c:a", "aac", "-shortest"},
		"audio": {"-f", "lavfi", "-i", "sine=f=440:d=2:sample_rate=44100"},
		"image": {"-f", "lavfi", "-i", "testsrc2=s=300x200", "-frames:v", "1"},
	}
	for k, args := range gen {
		cmd := command(tl.FFmpeg, append(append([]string{"-hide_banner", "-v", "error", "-y"}, args...), src[k])...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("make %s: %v %s", k, err, out)
		}
	}
	infos := map[string]*probeInfo{}
	for k, p := range src {
		info, err := tl.probe(p)
		if err != nil || info.Kind != k {
			t.Fatalf("probe %s: %v %+v", k, err, info)
		}
		infos[k] = info
	}

	for _, f := range tl.formats() {
		d := findFormat(f.ID)
		codecs := f.Codecs
		if len(codecs) == 0 {
			codecs = []Codec{{ID: "", OK: f.OK}}
		}
		for _, c := range codecs {
			for _, hw := range []bool{false, true} {
				if hw && !c.HW {
					continue
				}
				if !c.OK {
					t.Logf("skip %s/%s: no encoder", f.ID, c.ID)
					continue
				}
				inputs := []string{"video", "image"}
				if f.Kind == "audio" {
					inputs = []string{"video", "audio"}
				} else if f.Kind == "video" && c.ID != "gif" {
					inputs = []string{"video", "audio"}
				}
				for _, in := range inputs {
					if c.ID == "copy" && in != "video" {
						continue
					}
					name := f.ID + "-" + c.ID + "-" + in
					if hw {
						name += "-hw"
					}
					t.Run(name, func(t *testing.T) {
						s := defaultSettings()
						s.Format, s.HW, s.Res = f.ID, hw, 0
						if in == "video" && f.Kind != "audio" {
							s.Res = 144
						}
						out := filepath.Join(dir, name+"."+d.ext)
						args, err := buildArgs(tl, s, d, c.ID, infos[in], src[in], out)
						if err != nil {
							t.Fatal(err)
						}
						cmd := command(tl.FFmpeg, args...)
						var log ring
						if err := encode(cmd, &log, func() {}, func(progress) {}); err != nil {
							t.Fatalf("%v\n%s\n%v", err, quoteArgs(tl.FFmpeg, args), log.all())
						}
						if st, err := os.Stat(out); err != nil || st.Size() == 0 {
							t.Fatalf("no output: %v", err)
						}
						if _, err := tl.probe(out); err != nil {
							t.Fatalf("unreadable output: %v", err)
						}
					})
				}
			}
		}
	}
}

func TestSplitArgs(t *testing.T) {
	got := splitArgs(`-c:v libx264 -vf "scale=1280:-2, fps=30" -metadata title='a b'`)
	want := []string{"-c:v", "libx264", "-vf", "scale=1280:-2, fps=30", "-metadata", "title=a b"}
	if len(got) != len(want) {
		t.Fatalf("%q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%q", got)
		}
	}
}
