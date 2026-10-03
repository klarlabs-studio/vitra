package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// iconSize is the tray icon's edge in pixels: 18pt menu bar icons at 2x.
const iconSize = 36

// trayIcon draws the tray icon: a ring with the used share of the disk
// filled in, as a macOS template image (black with alpha, tinted by the
// system). Other platforms show it as drawn.
func trayIcon(usedPercent float64) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	c := float64(iconSize) / 2
	outer, inner := c-1, c-6
	used := math.Max(0, math.Min(usedPercent, 100)) / 100
	for y := range iconSize {
		for x := range iconSize {
			dx, dy := float64(x)+0.5-c, float64(y)+0.5-c
			r := math.Hypot(dx, dy)
			if r > outer {
				continue
			}
			// Angle clockwise from 12 o'clock, 0 to 1.
			a := math.Atan2(dx, -dy) / (2 * math.Pi)
			if a < 0 {
				a++
			}
			switch {
			case a < used:
				img.SetNRGBA(x, y, color.NRGBA{A: 0xff}) // used: solid
			case r >= inner:
				img.SetNRGBA(x, y, color.NRGBA{A: 0xff}) // free: ring only
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img) // encoding an in-memory NRGBA cannot fail
	return buf.Bytes()
}
