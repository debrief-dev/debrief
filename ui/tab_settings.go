package ui

import (
	"fmt"
	"image"
	"image/color"
	"log"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"
	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/infra/autostart"
	"github.com/debrief-dev/debrief/infra/hotkey"
)

// renderSettingsTab renders the settings view
//
//nolint:dupl // top-level layout is structurally similar to card internals but serves a different purpose
func renderSettingsTab(gtx C, app *appstate.State, theme *material.Theme) D {
	return material.List(theme, &app.SettingsList).Layout(gtx, 1, func(gtx C, _ int) D {
		return layout.Inset{
			Top:    SpacingXLarge,
			Bottom: SpacingXLarge,
			Left:   SpacingXLarge,
			Right:  SpacingXLarge,
		}.Layout(gtx, func(gtx C) D {
			return layout.Flex{
				Axis: layout.Vertical,
			}.Layout(gtx,
				// Hotkey configuration card
				layout.Rigid(func(gtx C) D {
					return renderHotkeyCard(gtx, app, theme)
				}),

				// Autostart card
				layout.Rigid(func(gtx C) D {
					return renderAutostartCard(gtx, app, theme)
				}),
			)
		})
	})
}

// renderHotkeyCard renders the hotkey configuration card
//
//nolint:dupl // label+inset patterns are structurally similar but have different content
func renderHotkeyCard(gtx C, app *appstate.State, theme *material.Theme) D {
	return renderCard(gtx, theme, HotKeyCardTitle, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Description
			layout.Rigid(func(gtx C) D {
				desc := material.Body2(theme, HotKeyCardDescription)
				desc.Color = color.NRGBA{R: ColorGray180, G: ColorGray180, B: ColorGray180, A: ColorWhite}

				return layout.Inset{Bottom: SpacingLarge}.Layout(gtx, desc.Layout)
			}),

			// Hotkey presets
			layout.Rigid(func(gtx C) D {
				return renderHotkeyPresets(gtx, app, theme)
			}),

			// Error/Success messages
			layout.Rigid(func(gtx C) D {
				return renderHotkeyMessages(gtx, app, theme)
			}),
		)
	})
}

// renderHotkeyPresets renders preset radio buttons for hotkey selection
//
//nolint:dupl // label+inset patterns are structurally similar but have different content
func renderHotkeyPresets(gtx C, app *appstate.State, theme *material.Theme) D {
	return layout.Inset{Bottom: SpacingLarge}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Label
			layout.Rigid(func(gtx C) D {
				label := material.Body2(theme, HotKeyCardAction)
				label.Color = color.NRGBA{R: ColorGray220, G: ColorGray220, B: ColorGray220, A: ColorWhite}

				return layout.Inset{Bottom: SpacingMedium}.Layout(gtx, label.Layout)
			}),
			// Preset buttons
			layout.Rigid(func(gtx C) D {
				presetButtons := make([]layout.FlexChild, len(app.Hotkeys.Presets))
				for i := range app.Hotkeys.Presets {
					presetID := i // Capture loop variable
					presetButtons[i] = layout.Rigid(func(gtx C) D {
						return renderPresetButton(gtx, app, theme, presetID)
					})
				}

				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, presetButtons...)
			}),
		)
	})
}

// renderPresetButton renders a single preset radio button
//
//nolint:dupl // preset and toggle buttons share visual structure but differ in behavior
func renderPresetButton(gtx C, app *appstate.State, theme *material.Theme, presetID int) D {
	preset := app.Hotkeys.Presets[presetID]
	isSelected := app.Hotkeys.SelectedPresetID == presetID
	clickable := &app.Hotkeys.PresetClickables[presetID]

	// Handle clicks - auto-save on selection
	for clickable.Clicked(gtx) {
		app.Hotkeys.SelectedPresetID = presetID
		app.Hotkeys.Error = ""
		app.Hotkeys.Success = false

		// Auto-save the selected hotkey
		saveHotkeyPreset(app)

		app.Window.Invalidate()
	}

	return layout.Inset{Bottom: SpacingMedium}.Layout(gtx, func(gtx C) D {
		return clickable.Layout(gtx, func(gtx C) D {
			// Colors based on selection/hover (matching badge pattern)
			bgColor := color.NRGBA{R: ColorDarkGray60, G: ColorDarkGray60, B: ColorDarkGray60, A: ColorGrayAlpha}
			textColor := color.NRGBA{R: ColorGray220, G: ColorGray220, B: ColorGray220, A: ColorWhite}

			if isSelected {
				bgColor = color.NRGBA{R: ColorBlueRed, G: ColorBlueGreen, B: ColorBlueBlue, A: ColorBlueAlpha}
				textColor = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}
			} else if clickable.Hovered() {
				bgColor = color.NRGBA{R: ColorDarkGray50, G: ColorDarkGray50, B: ColorDarkGray50, A: ColorWhite}
			}

			return layout.Background{}.Layout(gtx,
				// Rounded background
				func(gtx C) D {
					return drawRoundedBg(gtx, bgColor, BorderRadius)
				},
				// Content: circle + text
				func(gtx C) D {
					return layout.Inset{
						Top: SpacingMedium, Bottom: SpacingMedium,
						Left: SpacingLarge, Right: SpacingLarge,
					}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							// Circle indicator
							layout.Rigid(func(gtx C) D {
								return renderCircle(gtx, textColor)
							}),
							// Preset name + hotkey
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Left: SpacingLarge}.Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											label := material.Body1(theme, preset.Name)
											label.Color = textColor

											return label.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											label := material.Caption(theme, preset.DisplayName)
											label.Color = textColor
											label.Color.A = 200

											return layout.Inset{Top: SpacingXSmall}.Layout(gtx, label.Layout)
										}),
									)
								})
							}),
						)
					})
				},
			)
		})
	})
}

// renderCircle renders a simple circle indicator
func renderCircle(gtx C, clr color.NRGBA) D {
	size := gtx.Dp(CircleIndicatorSize)

	// Draw circle outline
	defer clip.Ellipse{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: size, Y: size},
	}.Push(gtx.Ops).Pop()

	paint.ColorOp{Color: clr}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	return D{Size: image.Pt(size, size)}
}

// renderHotkeyMessages shows error/success messages
func renderHotkeyMessages(gtx C, app *appstate.State, theme *material.Theme) D {
	if app.Hotkeys.Error != "" {
		return layout.Inset{Bottom: SpacingMedium}.Layout(gtx, func(gtx C) D {
			label := material.Body2(theme, app.Hotkeys.Error)
			label.Color = color.NRGBA{R: ColorErrorRed, G: ColorErrorGreen, B: ColorErrorBlue, A: ColorWhite}

			return label.Layout(gtx)
		})
	}

	if app.Hotkeys.Success && app.Hotkeys.SelectedPresetID >= 0 && app.Hotkeys.SelectedPresetID < len(app.Hotkeys.Presets) {
		return layout.Inset{Bottom: SpacingMedium}.Layout(gtx, func(gtx C) D {
			selectedPreset := app.Hotkeys.Presets[app.Hotkeys.SelectedPresetID]
			message := HotKeyCardSuccess + selectedPreset.DisplayName
			label := material.Body2(theme, message)
			label.Color = color.NRGBA{R: ColorSuccessRed, G: ColorSuccessGreen, B: ColorSuccessBlue, A: ColorWhite} // Green

			return label.Layout(gtx)
		})
	}

	return D{}
}

// renderAutostartCard renders the autostart toggle card.
//
//nolint:dupl // label+inset patterns are structurally similar but have different content
func renderAutostartCard(gtx C, app *appstate.State, theme *material.Theme) D {
	return renderCard(gtx, theme, AutoStartCardTitle, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Toggle button
			layout.Rigid(func(gtx C) D {
				return renderAutostartToggle(gtx, app, theme)
			}),

			// Error/Success messages
			layout.Rigid(func(gtx C) D {
				return renderAutostartMessages(gtx, app, theme)
			}),
		)
	})
}

// renderAutostartToggle renders the enabled/disabled checkbox toggle.
func renderAutostartToggle(gtx C, app *appstate.State, theme *material.Theme) D {
	clickable := &app.AutoStartClick

	for clickable.Clicked(gtx) {
		app.AutoStartError = ""
		app.AutoStartSuccess = false
		app.AutoStartNeedsUpdate = true
	}

	textColor := color.NRGBA{R: ColorGray220, G: ColorGray220, B: ColorGray220, A: ColorWhite}

	return layout.Inset{Bottom: SpacingLarge}.Layout(gtx, func(gtx C) D {
		return clickable.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return renderCheckbox(gtx, app.AutoStartEnabled)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: SpacingLarge}.Layout(gtx, func(gtx C) D {
						lbl := material.Body1(theme, AutoStartCardDescription)
						lbl.Color = textColor

						return lbl.Layout(gtx)
					})
				}),
			)
		})
	})
}

// renderCheckbox draws a square checkbox indicator with an X mark when checked.
func renderCheckbox(gtx C, checked bool) D {
	size := gtx.Dp(CheckboxSize)
	rr := gtx.Dp(CheckboxRadius)

	// Draw box background
	rect := clip.RRect{
		Rect: image.Rectangle{Max: image.Pt(size, size)},
		NE:   rr, NW: rr, SE: rr, SW: rr,
	}

	stack := rect.Push(gtx.Ops)

	if checked {
		paint.Fill(gtx.Ops, color.NRGBA{R: ColorBlueRed, G: ColorBlueGreen, B: ColorBlueBlue, A: ColorWhite})
	} else {
		paint.Fill(gtx.Ops, color.NRGBA{R: ColorDarkGray60, G: ColorDarkGray60, B: ColorDarkGray60, A: ColorWhite})
	}

	stack.Pop()

	if checked {
		drawXMark(gtx, size)
	}

	return D{Size: image.Pt(size, size)}
}

// drawXMark draws two diagonal stroked lines forming an X inside a box.
func drawXMark(gtx C, size int) {
	margin := float32(size) / CheckboxMarginDivisor
	width := float32(size) / CheckboxStrokeDivisor
	clr := color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}

	// Top-left to bottom-right
	drawStrokeLine(gtx, f32.Pt(margin, margin), f32.Pt(float32(size)-margin, float32(size)-margin), width, clr)
	// Top-right to bottom-left
	drawStrokeLine(gtx, f32.Pt(float32(size)-margin, margin), f32.Pt(margin, float32(size)-margin), width, clr)
}

// drawStrokeLine draws a single stroked line between two points.
func drawStrokeLine(gtx C, from, to f32.Point, width float32, clr color.NRGBA) {
	var path clip.Path

	path.Begin(gtx.Ops)
	path.MoveTo(from)
	path.LineTo(to)

	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, clr)
	stack.Pop()
}

// renderAutostartMessages shows error/success messages for the autostart toggle.
func renderAutostartMessages(gtx C, app *appstate.State, theme *material.Theme) D {
	if app.AutoStartError != "" {
		return layout.Inset{Bottom: SpacingMedium}.Layout(gtx, func(gtx C) D {
			label := material.Body2(theme, app.AutoStartError)
			label.Color = color.NRGBA{R: ColorErrorRed, G: ColorErrorGreen, B: ColorErrorBlue, A: ColorWhite}

			return label.Layout(gtx)
		})
	}

	if app.AutoStartSuccess {
		return layout.Inset{Bottom: SpacingMedium}.Layout(gtx, func(gtx C) D {
			label := material.Body2(theme, AutoStartSuccess)
			label.Color = color.NRGBA{R: ColorSuccessRed, G: ColorSuccessGreen, B: ColorSuccessBlue, A: ColorWhite}

			return label.Layout(gtx)
		})
	}

	return D{}
}

// ProcessAutostartUpdate performs the actual autostart toggle and config save.
// Must be called after ev.Frame in a goroutine to avoid blocking the UI thread.
func ProcessAutostartUpdate(app *appstate.State) {
	newState := !app.AutoStartEnabled

	var err error
	if newState {
		err = autostart.Enable()
	} else {
		err = autostart.Disable()
	}

	if err != nil {
		app.AutoStartError = fmt.Sprintf(AutoStartFailure, err)
		log.Printf("Autostart toggle failed: %v", err)

		app.AutoStartUpdating = false
		app.Window.Invalidate()

		return
	}

	app.AutoStartEnabled = newState
	app.Config.AutoStart = newState

	if err := app.Config.SaveConfig(app.ConfigPath); err != nil {
		app.AutoStartError = fmt.Sprintf(AutoStartFailureCfg, err)
		log.Printf("Config save failed after autostart toggle: %v", err)
	} else {
		app.AutoStartSuccess = true
	}

	app.AutoStartUpdating = false
	app.Window.Invalidate()
}

// saveHotkeyPreset schedules a hotkey update for after frame submission.
// Actual registration is deferred to ProcessHotkeyUpdate to avoid a
// dispatch_sync deadlock on macOS: the hotkey library calls
// dispatch_sync(main_queue) internally, which deadlocks when invoked
// before ev.Frame.
func saveHotkeyPreset(app *appstate.State) {
	app.Hotkeys.Error = ""
	app.Hotkeys.Success = false
	app.Hotkeys.NeedsUpdate = true
}

// ProcessHotkeyUpdate performs the actual hotkey registration and config save.
// Must be called after ev.Frame to avoid a dispatch_sync deadlock on macOS.
func ProcessHotkeyUpdate(app *appstate.State) {
	if app.Hotkeys.SelectedPresetID < 0 || app.Hotkeys.SelectedPresetID >= len(app.Hotkeys.Presets) {
		app.Hotkeys.Error = "Invalid preset selection"
		app.Window.Invalidate()

		return
	}

	preset := app.Hotkeys.Presets[app.Hotkeys.SelectedPresetID]

	mods, key, err := hotkey.ConvertStrings(preset.Modifiers, preset.Key)
	if err != nil {
		app.Hotkeys.Error = err.Error()
		app.Window.Invalidate()

		return
	}

	if err := app.Hotkeys.Manager.UpdateHotkey(mods, key, preset.Modifiers, preset.Key); err != nil {
		app.Hotkeys.Error = fmt.Sprintf(HotKeyCardFailure, err)
		log.Printf("Hotkey registration failed: %v", err)
		app.Window.Invalidate()

		return
	}

	app.Config.HotkeyPreset = app.Hotkeys.SelectedPresetID

	if err := app.Config.SaveConfig(app.ConfigPath); err != nil {
		app.Hotkeys.Error = fmt.Sprintf(HotKeyCardFailureDueToCfg, err)
		log.Printf("Config save failed: %v", err)
	} else {
		app.Hotkeys.Success = true

		log.Printf("Hotkey updated: %v + %v", mods, key)
	}

	app.Window.Invalidate()
}
