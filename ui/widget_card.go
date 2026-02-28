package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// drawRoundedBg draws a rounded rectangle background with the specified color and corner radius.
// Shared by badges, tabs, cards, settings presets, and statistics items.
func drawRoundedBg(gtx C, bgColor color.NRGBA, radius unit.Dp) D {
	size := gtx.Constraints.Min
	rr := gtx.Dp(radius)
	rect := clip.RRect{
		Rect: image.Rectangle{Max: size},
		NE:   rr, NW: rr, SE: rr, SW: rr,
	}
	stack := rect.Push(gtx.Ops)
	paint.Fill(gtx.Ops, bgColor)
	stack.Pop()

	return D{Size: size}
}

// drawSelectionBg draws a selection or hover highlight behind a list item.
// Shared by the Commands and Tree View tabs.
func drawSelectionBg(gtx C, size image.Point, isSelected, isHovered bool) {
	if isSelected {
		stack := clip.Rect{Max: size}.Push(gtx.Ops)
		paint.Fill(gtx.Ops, color.NRGBA{R: ColorBlueRed, G: ColorBlueGreen, B: ColorBlueBlue, A: ColorWhite})
		stack.Pop()
	} else if isHovered {
		stack := clip.Rect{Max: size}.Push(gtx.Ops)
		paint.Fill(gtx.Ops, color.NRGBA{R: ColorDarkGray50, G: ColorDarkGray50, B: ColorDarkGray50, A: ColorGrayAlpha40})
		stack.Pop()
	}
}

// renderCard renders a card with title and custom content (shared UI component)
//
//nolint:dupl // card title pattern is structurally similar to settings description labels
func renderCard(gtx C, theme *material.Theme, title string, content layout.Widget) D {
	return layout.Inset{Bottom: SpacingXLarge}.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				return drawRoundedBg(gtx, color.NRGBA{R: ColorDarkGray40, G: ColorDarkGray40, B: ColorDarkGray40, A: ColorWhite}, SpacingMedium)
			},
			func(gtx C) D {
				return layout.Inset{
					Top:    SpacingLarge,
					Bottom: SpacingLarge,
					Left:   SpacingXLarge,
					Right:  SpacingXLarge,
				}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						// Card title
						layout.Rigid(func(gtx C) D {
							cardTitle := material.H6(theme, title)
							cardTitle.Color = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}

							return layout.Inset{Bottom: SpacingLarge}.Layout(gtx, cardTitle.Layout)
						}),
						// Content
						layout.Rigid(content),
					)
				})
			},
		)
	})
}
