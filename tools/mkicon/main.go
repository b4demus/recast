// mkicon draws build/appicon.png: Glyph's ramp in one picture — three faint
// bars (≡) turning into a solid clay block (■) on graphite.
// Wails makes build/windows/icon.ico from it when the .ico is missing.
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

const (
	size = 1024
	ss   = 4 // supersampling per axis
)

var (
	bg    = color.RGBA{20, 20, 20, 255}
	frame = color.RGBA{107, 103, 98, 255}
	ink   = color.RGBA{154, 149, 140, 255}
	clay  = color.RGBA{187, 116, 99, 255}
)

type rect struct {
	x0, y0, x1, y1 float64
	c              color.RGBA
}

func main() {
	// coordinates in a 0..100 box
	shapes := []rect{
		{4, 4, 96, 96, frame},
		{6, 6, 94, 94, bg},
		// ≡
		{16, 33, 40, 39, ink},
		{16, 47, 40, 53, ink},
		{16, 61, 40, 67, ink},
		// ->
		{45, 48, 53, 52, frame},
		// ■
		{58, 30, 86, 70, clay},
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := (float64(x) + (float64(sx)+.5)/ss) * 100 / size
					py := (float64(y) + (float64(sy)+.5)/ss) * 100 / size
					for i := len(shapes) - 1; i >= 0; i-- {
						s := shapes[i]
						if px >= s.x0 && px < s.x1 && py >= s.y0 && py < s.y1 {
							r, g, b, a = r+float64(s.c.R), g+float64(s.c.G), b+float64(s.c.B), a+255
							break
						}
					}
				}
			}
			n := float64(ss * ss)
			img.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), uint8(a / n)})
		}
	}
	os.MkdirAll("build", 0o755)
	f, err := os.Create("build/appicon.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
