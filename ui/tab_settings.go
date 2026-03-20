package ui

import (
	"fmt"
	"image"
	"image/color"
	"log"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"
	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/infra/hotkey"
)

// renderSettingsTab renders the settings view
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
				// Title
				layout.Rigid(func(gtx C) D {
					title := material.H5(theme, "Settings")
					return layout.Inset{Bottom: SpacingHuge}.Layout(gtx, title.Layout)
				}),

				// Hotkey configuration card
				layout.Rigid(func(gtx C) D {
					return renderHotkeyCard(gtx, app, theme)
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
			label.Color = color.NRGBA{R: ColorErrorRed, G: ColorErrorGreen, B: ColorErrorGreen, A: ColorWhite} // Red

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
