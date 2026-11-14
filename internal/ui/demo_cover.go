package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"
)

const demoCoverKey = "demo-tree-of-clues"

var demoCovers = sync.OnceValue(func() map[string][]byte {
	return map[string][]byte{demoCoverKey: drawDemoCover()}
})

func (d *Demo) NowCover() string { return demoCoverKey }

func (d *Demo) Cover(key string) ([]byte, string, bool) {
	data, ok := demoCovers()[key]
	return data, "image/png", ok
}

func drawDemoCover() []byte {
	const n, cell = 32, 10
	sky1, sky2 := color.RGBA{0x2B, 0x0F, 0x3A, 255}, color.RGBA{0xE2, 0x55, 0x9A, 255}
	sun1, sun2 := color.RGBA{0xFF, 0xE0, 0x7A, 255}, color.RGBA{0xFF, 0x6F, 0x61, 255}
	ground, grid := color.RGBA{0x1A, 0x0B, 0x24, 255}, color.RGBA{0xFF, 0x7B, 0xE0, 255}
	mix := func(a, b color.RGBA, t float64) color.RGBA {
		f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
		return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
	}
	const horizon = 21
	img := image.NewRGBA(image.Rect(0, 0, n*cell, n*cell))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var c color.RGBA
			if y < horizon {
				c = mix(sky1, sky2, float64(y)/horizon)
				if (x*7+y*13)%37 == 0 && y < 12 {
					c = color.RGBA{0xFF, 0xF4, 0xFC, 255}
				}
				dx, dy := float64(x)-15.5, float64(y)-17.5
				if dx*dx+dy*dy < 90 && !(y > 15 && y%2 == 0) {
					c = mix(sun1, sun2, float64(y-8)/12)
				}
			} else {
				c = ground
				d := y - horizon + 1
				if d == 1 || d == 3 || d == 6 || d == 10 {
					c = grid
				}
				for k := -7; k <= 7; k++ {
					if int(math.Floor(15.5+float64(k)*(1.2+0.75*float64(d)))) == x {
						c = grid
					}
				}
			}
			for py := 0; py < cell; py++ {
				for px := 0; px < cell; px++ {
					img.SetRGBA(x*cell+px, y*cell+py, c)
				}
			}
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}
