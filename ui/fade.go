package ui

import (
	"image"
	"image/color"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// renderFadeGradient overlays a horizontal fade-out gradient (transparent -> bgColor)
// across the specified region. Used to truncate overflowing text with a smooth fade.
func renderFadeGradient(ops *op.Ops, fadeStart, fadeWidthPx, steps, clippedHeight int, bgColor color.NRGBA) {
	for i := range steps {
		x0 := fadeStart + (fadeWidthPx * i / steps)
		x1 := fadeStart + (fadeWidthPx * (i + 1) / steps)
		alphaInt := ColorWhite * (i + 1) / steps
		alpha := uint8(alphaInt) //nolint:gosec // alphaInt is always 0-255

		stack := clip.Rect{
			Min: image.Pt(x0, 0),
			Max: image.Pt(x1, clippedHeight),
		}.Push(ops)
		paint.Fill(ops, color.NRGBA{R: bgColor.R, G: bgColor.G, B: bgColor.B, A: alpha})
		stack.Pop()
	}
}
