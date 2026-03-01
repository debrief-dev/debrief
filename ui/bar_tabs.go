package ui

import (
	"image"
	"image/color"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"
	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/model"
)

// renderTabBar renders the tab navigation bar at the top
func renderTabBar(gtx C, app *appstate.State, theme *material.Theme) D {
	return layout.Inset{
		Top:    SpacingMedium,
		Bottom: SpacingMedium,
		Left:   SpacingMedium,
		Right:  SpacingMedium,
	}.Layout(gtx, func(gtx C) D {
		// Make the entire tab bar area draggable
		return layout.Stack{}.Layout(gtx,
			// Background draggable layer (invisible)
			layout.Stacked(func(gtx C) D {
				dragHeight := gtx.Dp(dragAreaHeight)
				dragSize := image.Point{X: gtx.Constraints.Max.X, Y: dragHeight}

				// Make this area draggable
				stack := clip.Rect{Max: dragSize}.Push(gtx.Ops)
				system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
				stack.Pop()

				return D{Size: dragSize}
			}),
			// Tabs on top
			layout.Stacked(func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(gtx,
					// Commands tab
					layout.Rigid(func(gtx C) D {
						return renderTab(gtx, app, theme, model.TabCommands, &app.Tabs.CommandsTab)
					}),
					// Tree tab
					layout.Rigid(func(gtx C) D {
						return renderTab(gtx, app, theme, model.TabTree, &app.Tabs.TreeTab)
					}),
					// Spacer to push Settings and Statistics to the right
					layout.Flexed(1.0, func(gtx C) D {
						return D{Size: gtx.Constraints.Min}
					}),
					// Top Commands tab
					layout.Rigid(func(gtx C) D {
						return renderTab(gtx, app, theme, model.TabTopCommands, &app.Tabs.StatisticsTab)
					}),
					// Settings tab
					layout.Rigid(func(gtx C) D {
						return renderTab(gtx, app, theme, model.TabSettings, &app.Tabs.SettingsTab)
					}),
				)
			}),
		)
	})
}

// renderTab renders a single tab button with hint below
func renderTab(gtx C, app *appstate.State, theme *material.Theme, tabType model.Tab, clickable *widget.Clickable) D {
	for clickable.Clicked(gtx) {
		switchToTab(app, tabType)
	}

	return layout.Inset{Right: SpacingXSmall}.Layout(gtx, func(gtx C) D {
		// Render button and hint in vertical layout
		return layout.Flex{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// Tab button
			layout.Rigid(func(gtx C) D {
				return material.Clickable(gtx, clickable, func(gtx C) D {
					// Tab styling
					bgColor := color.NRGBA{R: ColorDarkGray40, G: ColorDarkGray40, B: ColorDarkGray40, A: ColorWhite}
					textColor := color.NRGBA{R: ColorGray180, G: ColorGray180, B: ColorGray180, A: ColorWhite}

					if app.Tabs.Current == tabType {
						bgColor = color.NRGBA{R: ColorBlueRed, G: ColorBlueGreen, B: ColorBlueBlue, A: ColorWhite}
						textColor = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}
					}

					return layout.Background{}.Layout(gtx,
						func(gtx C) D {
							return drawRoundedBg(gtx, bgColor, BorderRadius)
						},
						func(gtx C) D {
							return layout.Inset{
								Top:    SpacingMedium,
								Bottom: SpacingMedium,
								Left:   SpacingXLarge,
								Right:  SpacingXLarge,
							}.Layout(gtx, func(gtx C) D {
								label := material.Body1(theme, tabName(tabType))
								label.Color = textColor

								return label.Layout(gtx)
							})
						},
					)
				})
			}),
			// Keyboard shortcut hint below button
			layout.Rigid(func(gtx C) D {
				hintLabel := material.Caption(theme, tabShortcutHint(tabType))
				hintLabel.Color = color.NRGBA{R: ColorGray180, G: ColorGray180, B: ColorGray180, A: ColorWhite}

				return layout.Inset{Top: SpacingTiny}.Layout(gtx, hintLabel.Layout)
			}),
		)
	})
}
