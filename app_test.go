package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestQueue runs files through the real scheduler: probe, start, two at once,
// stop and requeue, convert again, unique output names.
func TestQueue(t *testing.T) {
	tl := loadTools("")
	if !tl.ok() {
		t.Skip("ffmpeg not found")
	}
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mkv")
	long := filepath.Join(dir, "long.mkv")
	junk := filepath.Join(dir, "notes.txt")
	for p, d := range map[string]string{clip: "2", long: "60"} {
		cmd := command(tl.FFmpeg, "-hide_banner", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=320x240:r=25:d="+d,
			"-f", "lavfi", "-i", "sine=d="+d, "-c:v", "libx264", "-c:a", "aac", "-shortest", p)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	os.WriteFile(junk, []byte("hello"), 0o644)

	a := &App{ready: make(chan struct{}), reserved: map[string]bool{}, probeSem: make(chan struct{}, 4), tools: tl, dataDir: dir}
	a.data.Settings = defaultSettings()
	a.data.Settings.Parallel = 2
	a.data.Settings.Res = 144
	close(a.ready)

	wait := func(cond func() bool) {
		t.Helper()
		for end := time.Now().Add(60 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
			a.mu.Lock()
			ok := cond()
			a.mu.Unlock()
			if ok {
				return
			}
		}
		t.Fatalf("timeout: %+v", a.GetState().Jobs)
	}
	status := func(name string) string {
		for _, j := range a.jobs {
			if j.Name == name {
				return j.Status
			}
		}
		return ""
	}

	if n := a.AddPaths([]string{clip, long, junk, clip}); n != 3 {
		t.Fatalf("added %d, want 3 (duplicate skipped)", n)
	}
	wait(func() bool { return status("clip.mkv") == "ready" && status("long.mkv") == "ready" && status("notes.txt") == "error" })

	// stop while running: the files go back to waiting, no half-written output stays
	a.data.Settings.Format = "webm" // slow enough to catch it running
	a.Start()
	wait(func() bool { return status("long.mkv") == "running" })
	time.Sleep(300 * time.Millisecond)
	a.Stop()
	wait(func() bool { return status("long.mkv") == "ready" })
	if _, err := os.Stat(filepath.Join(dir, "long.webm")); err == nil {
		t.Fatal("partial output left behind")
	}

	a.data.Settings.Format = "mp4"
	a.Start()
	wait(func() bool { return !a.running })
	for _, name := range []string{"clip.mp4", "long.mp4"} {
		if st, err := os.Stat(filepath.Join(dir, name)); err != nil || st.Size() == 0 {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if status("clip.mkv") != "done" || a.find("1").Progress != 1 {
		t.Fatalf("%+v", a.GetState().Jobs)
	}

	// nothing waiting: Start converts the finished files again under new names
	a.Start()
	wait(func() bool { return !a.running })
	if _, err := os.Stat(filepath.Join(dir, "clip (1).mp4")); err != nil {
		t.Fatal("second run did not pick a free name:", err)
	}
}
