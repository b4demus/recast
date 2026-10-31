package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Codec is one way to encode a format's main stream. OK is false when this
// ffmpeg build has neither a software nor a working GPU encoder for it.
type Codec struct {
	ID string `json:"id"`
	OK bool   `json:"ok"`
	HW bool   `json:"hw"` // a working GPU encoder exists
}

// Format is an output target the UI offers. The first codec is the default.
type Format struct {
	ID       string  `json:"id"`
	Ext      string  `json:"ext"`
	Kind     string  `json:"kind"` // video | audio | image
	Lossless bool    `json:"lossless"`
	Codecs   []Codec `json:"codecs"`
	OK       bool    `json:"ok"`
}

type formatDef struct {
	id, ext, kind string
	lossless      bool
	codecs        []string
	need          string // encoder for audio/image formats
}

var catalog = []formatDef{
	{id: "mp4", ext: "mp4", kind: "video", codecs: []string{"h264", "h265", "av1", "copy"}},
	{id: "mkv", ext: "mkv", kind: "video", codecs: []string{"h264", "h265", "av1", "copy"}},
	{id: "mov", ext: "mov", kind: "video", codecs: []string{"h264", "h265", "prores", "copy"}},
	{id: "webm", ext: "webm", kind: "video", codecs: []string{"vp9", "av1"}},
	{id: "avi", ext: "avi", kind: "video", codecs: []string{"mpeg4"}},
	{id: "m4v", ext: "m4v", kind: "video", codecs: []string{"h264"}}, // ipod muxer: no HEVC
	{id: "wmv", ext: "wmv", kind: "video", codecs: []string{"wmv2"}},
	{id: "flv", ext: "flv", kind: "video", codecs: []string{"h264"}},
	{id: "ts", ext: "ts", kind: "video", codecs: []string{"h264", "h265", "copy"}},
	{id: "mpg", ext: "mpg", kind: "video", codecs: []string{"mpeg2"}},
	{id: "ogv", ext: "ogv", kind: "video", codecs: []string{"theora"}},
	{id: "gif", ext: "gif", kind: "video", codecs: []string{"gif"}},

	{id: "mp3", ext: "mp3", kind: "audio", need: "libmp3lame"},
	{id: "m4a", ext: "m4a", kind: "audio", need: "aac"},
	{id: "aac", ext: "aac", kind: "audio", need: "aac"},
	{id: "wav", ext: "wav", kind: "audio", need: "pcm_s16le", lossless: true},
	{id: "flac", ext: "flac", kind: "audio", need: "flac", lossless: true},
	{id: "ogg", ext: "ogg", kind: "audio", need: "libvorbis"},
	{id: "opus", ext: "opus", kind: "audio", need: "libopus"},
	{id: "wma", ext: "wma", kind: "audio", need: "wmav2"},
	{id: "aiff", ext: "aiff", kind: "audio", need: "pcm_s16be", lossless: true},
	{id: "ac3", ext: "ac3", kind: "audio", need: "ac3"},
	{id: "alac", ext: "m4a", kind: "audio", need: "alac", lossless: true},

	{id: "png", ext: "png", kind: "image", need: "png", lossless: true},
	{id: "jpg", ext: "jpg", kind: "image", need: "mjpeg"},
	{id: "webp", ext: "webp", kind: "image", need: "libwebp"},
	{id: "avif", ext: "avif", kind: "image", need: "libaom-av1"},
	{id: "bmp", ext: "bmp", kind: "image", need: "bmp", lossless: true},
	{id: "tiff", ext: "tiff", kind: "image", need: "tiff", lossless: true},
	{id: "ico", ext: "ico", kind: "image", need: "png", lossless: true},
}

// Software encoder behind each video codec id.
var swEncoder = map[string]string{
	"h264": "libx264", "h265": "libx265", "av1": "libsvtav1", "vp9": "libvpx-vp9",
	"prores": "prores_ks", "mpeg4": "mpeg4", "wmv2": "wmv2", "mpeg2": "mpeg2video",
	"theora": "libtheora", "gif": "gif",
}

// GPU encoders per codec, in order of preference.
var hwEncoders = map[string][]string{
	"h264": {"h264_nvenc", "h264_qsv", "h264_amf"},
	"h265": {"hevc_nvenc", "hevc_qsv", "hevc_amf"},
	"av1":  {"av1_nvenc", "av1_qsv", "av1_amf"},
}

func findFormat(id string) *formatDef {
	for i := range catalog {
		if catalog[i].id == id {
			return &catalog[i]
		}
	}
	return nil
}

// formats is the catalog as this ffmpeg build can serve it.
func (t *tools) formats() []Format {
	out := make([]Format, 0, len(catalog))
	for _, d := range catalog {
		// Codecs is [] rather than null in JSON: the UI reads its length
		f := Format{ID: d.id, Ext: d.ext, Kind: d.kind, Lossless: d.lossless, Codecs: []Codec{}}
		if d.need != "" {
			f.OK = t.has(d.need)
		}
		for _, c := range d.codecs {
			hw := t.hw[c] != ""
			ok := c == "copy" || t.has(swEncoder[c]) || hw
			f.Codecs = append(f.Codecs, Codec{ID: c, OK: ok, HW: hw})
			f.OK = f.OK || ok
		}
		out = append(out, f)
	}
	return out
}

func pick[T any](q int, v ...T) T { return v[q] }

func qIndex(s string) int {
	switch s {
	case "high":
		return 0
	case "low":
		return 2
	}
	return 1
}

var errNoVideo = errors.New("e.noVideo")

// buildArgs turns the settings into an ffmpeg command line for one file.
// Progress goes to stdout as key=value lines, warnings and errors to stderr.
func buildArgs(t *tools, s Settings, d *formatDef, codec string, in *probeInfo, inPath, outPath string) ([]string, error) {
	q := qIndex(s.Quality)
	a := []string{"-hide_banner", "-nostdin", "-y", "-v", "warning", "-progress", "pipe:1", "-nostats"}
	stillOut := d.kind == "image" && s.CustomExt == ""
	if stillOut && in.Kind == "video" && in.Duration > 1 {
		// one frame from the middle: the first one is often black
		a = append(a, "-ss", strconv.FormatFloat(in.Duration/2, 'f', 3, 64))
	}
	a = append(a, "-i", inPath)

	// Own parameters replace everything we would pick; an own extension alone
	// leaves the codecs to ffmpeg's defaults for that container.
	if strings.TrimSpace(s.CustomArgs) != "" {
		return append(append(a, splitArgs(s.CustomArgs)...), outPath), nil
	}
	if s.CustomExt != "" {
		return append(a, outPath), nil
	}

	switch d.kind {
	case "audio":
		if !in.HasAudio {
			return nil, errors.New("e.noAudio")
		}
		a = append(a, "-vn", "-sn", "-dn")
		a = append(a, audioArgs(d.id, q)...)
	case "image":
		if !in.HasVideo {
			return nil, errNoVideo
		}
		a = append(a, "-an", "-sn", "-dn", "-frames:v", "1", "-update", "1")
		a = append(a, imageArgs(d.id, q, s.Res)...)
	default:
		v, err := videoArgs(t, s, d, codec, q, in)
		if err != nil {
			return nil, err
		}
		a = append(a, v...)
	}
	return append(a, outPath), nil
}

func videoArgs(t *tools, s Settings, d *formatDef, codec string, q int, in *probeInfo) ([]string, error) {
	mp4ish := d.id == "mp4" || d.id == "mov" || d.id == "m4v"
	var a []string
	if codec == "copy" {
		if d.id == "mkv" {
			a = append(a, "-map", "0", "-c", "copy")
		} else {
			a = append(a, "-c", "copy", "-sn", "-dn")
		}
		if mp4ish {
			a = append(a, "-movflags", "+faststart")
		}
		return a, nil
	}

	if codec == "gif" {
		if !in.HasVideo {
			return nil, errNoVideo
		}
		scale := ""
		if s.Res > 0 {
			scale = fmt.Sprintf("scale=-1:min(%d\\,ih):flags=lanczos,", s.Res)
		}
		vf := fmt.Sprintf("fps=%d,%ssplit[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=5",
			pick(q, 20, 15, 10), scale)
		return []string{"-vf", vf, "-an", "-sn", "-dn", "-loop", "0"}, nil
	}

	if in.HasVideo {
		a = append(a, videoCodecArgs(t, s.HW, codec, q)...)
		// 4:2:0 codecs need even sides; never upscale
		vf := "scale=trunc(iw/2)*2:trunc(ih/2)*2"
		if s.Res > 0 {
			vf = fmt.Sprintf("scale=-2:trunc(min(%d\\,ih)/2)*2", s.Res)
		}
		a = append(a, "-vf", vf)
		if codec == "h265" && mp4ish {
			a = append(a, "-tag:v", "hvc1") // so Apple players accept it
		}
	} else {
		a = append(a, "-vn")
	}
	if in.HasAudio {
		a = append(a, containerAudio(d.id, q)...)
	}
	a = append(a, "-sn", "-dn")
	if mp4ish {
		a = append(a, "-movflags", "+faststart")
	}
	return a, nil
}

func videoCodecArgs(t *tools, hw bool, codec string, q int) []string {
	if enc := t.hw[codec]; enc != "" && (hw || !t.has(swEncoder[codec])) {
		return hwCodecArgs(enc, q)
	}
	switch codec {
	case "h264":
		return []string{"-c:v", "libx264", "-preset", "medium", "-crf", pick(q, "18", "23", "28"), "-pix_fmt", "yuv420p"}
	case "h265":
		return []string{"-c:v", "libx265", "-preset", "medium", "-crf", pick(q, "20", "26", "31"), "-pix_fmt", "yuv420p", "-x265-params", "log-level=error"}
	case "av1":
		return []string{"-c:v", "libsvtav1", "-preset", "6", "-crf", pick(q, "26", "34", "42"), "-pix_fmt", "yuv420p"}
	case "vp9":
		return []string{"-c:v", "libvpx-vp9", "-crf", pick(q, "28", "34", "40"), "-b:v", "0", "-row-mt", "1", "-deadline", "good", "-cpu-used", "2", "-pix_fmt", "yuv420p"}
	case "prores":
		return []string{"-c:v", "prores_ks", "-profile:v", pick(q, "3", "2", "0"), "-pix_fmt", "yuv422p10le"}
	case "mpeg4":
		return []string{"-c:v", "mpeg4", "-q:v", pick(q, "2", "4", "7"), "-tag:v", "XVID", "-pix_fmt", "yuv420p"}
	case "wmv2":
		return []string{"-c:v", "wmv2", "-q:v", pick(q, "2", "5", "9")}
	case "mpeg2":
		return []string{"-c:v", "mpeg2video", "-q:v", pick(q, "2", "4", "7"), "-pix_fmt", "yuv420p"}
	case "theora":
		return []string{"-c:v", "libtheora", "-q:v", pick(q, "8", "6", "4")}
	}
	return nil
}

func hwCodecArgs(enc string, q int) []string {
	switch {
	case strings.HasSuffix(enc, "_nvenc"):
		return []string{"-c:v", enc, "-preset", "p5", "-rc", "vbr", "-cq", pick(q, "19", "24", "30"), "-b:v", "0", "-pix_fmt", "yuv420p"}
	case strings.HasSuffix(enc, "_qsv"):
		return []string{"-c:v", enc, "-global_quality", pick(q, "20", "25", "31"), "-pix_fmt", "nv12"}
	default: // _amf
		qp := pick(q, "20", "25", "31")
		a := []string{"-c:v", enc, "-rc", "cqp", "-qp_i", qp, "-qp_p", qp}
		if !strings.HasPrefix(enc, "av1") {
			a = append(a, "-qp_b", qp)
		}
		return append(a, "-pix_fmt", "nv12")
	}
}

// containerAudio is the audio track that each video container expects.
func containerAudio(id string, q int) []string {
	switch id {
	case "webm":
		return []string{"-c:a", "libopus", "-b:a", pick(q, "160k", "128k", "96k")}
	case "ogv":
		return []string{"-c:a", "libvorbis", "-q:a", pick(q, "7", "5", "3")}
	case "avi":
		return []string{"-c:a", "libmp3lame", "-q:a", pick(q, "0", "2", "5")}
	case "wmv":
		return []string{"-c:a", "wmav2", "-b:a", pick(q, "192k", "160k", "128k")}
	case "mpg":
		return []string{"-c:a", "mp2", "-b:a", pick(q, "320k", "256k", "192k")}
	}
	return []string{"-c:a", "aac", "-b:a", pick(q, "256k", "192k", "128k")}
}

func audioArgs(id string, q int) []string {
	switch id {
	case "mp3":
		return []string{"-c:a", "libmp3lame", "-q:a", pick(q, "0", "2", "5")}
	case "m4a":
		return []string{"-c:a", "aac", "-b:a", pick(q, "256k", "192k", "128k"), "-movflags", "+faststart"}
	case "aac":
		return []string{"-c:a", "aac", "-b:a", pick(q, "256k", "192k", "128k")}
	case "wav":
		return []string{"-c:a", "pcm_s16le"}
	case "flac":
		return []string{"-c:a", "flac", "-compression_level", "8"}
	case "ogg":
		return []string{"-c:a", "libvorbis", "-q:a", pick(q, "7", "5", "3")}
	case "opus":
		return []string{"-c:a", "libopus", "-b:a", pick(q, "160k", "128k", "96k")}
	case "wma":
		return []string{"-c:a", "wmav2", "-b:a", pick(q, "192k", "160k", "128k")}
	case "aiff":
		return []string{"-c:a", "pcm_s16be"}
	case "ac3":
		return []string{"-c:a", "ac3", "-b:a", pick(q, "448k", "384k", "192k")}
	case "alac":
		return []string{"-c:a", "alac", "-movflags", "+faststart"}
	}
	return nil
}

func imageArgs(id string, q, res int) []string {
	var a []string
	switch {
	case id == "ico":
		a = append(a, "-vf", "scale=256:256:force_original_aspect_ratio=decrease")
	case res > 0:
		a = append(a, "-vf", fmt.Sprintf("scale=-1:min(%d\\,ih)", res))
	}
	switch id {
	case "jpg":
		return append(a, "-c:v", "mjpeg", "-q:v", pick(q, "2", "4", "8"))
	case "webp":
		return append(a, "-c:v", "libwebp", "-quality", pick(q, "92", "80", "65"))
	case "avif":
		return append(a, "-c:v", "libaom-av1", "-still-picture", "1", "-crf", pick(q, "18", "28", "38"), "-cpu-used", "6", "-pix_fmt", "yuv420p")
	case "bmp":
		return append(a, "-c:v", "bmp")
	case "tiff":
		return append(a, "-c:v", "tiff")
	case "ico":
		return append(a, "-c:v", "png", "-pix_fmt", "rgba") // the ico muxer takes only RGBA PNG
	}
	return append(a, "-c:v", "png")
}

// splitArgs splits a command line the way a shell would for plain words and
// "double" or 'single' quoted parts.
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	inArg, quote := false, rune(0)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inArg = r, true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inArg {
				out = append(out, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if inArg {
		out = append(out, cur.String())
	}
	return out
}
