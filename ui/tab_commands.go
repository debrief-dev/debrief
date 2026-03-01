package ui

import (
	"fmt"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget/material"
	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/model"
	"github.com/debrief-dev/debrief/tree"
)

// renderCommandsTab renders the existing commands view
//
//nolint:dupl // label+inset patterns are structurally similar but have different content
func renderCommandsTab(gtx C, app *appstate.State, theme *material.Theme) D {
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Command list (takes remaining space)
		layout.Flexed(1.0, func(gtx C) D {
			return renderCommandList(gtx, app, theme)
		}),
		// Source filter bar (fixed above search)
		layout.Rigid(func(gtx C) D {
			return renderShellFilterBar(gtx, app, theme)
		}),
		// Search input (fixed at bottom)
		layout.Rigid(func(gtx C) D {
			return renderSearchInput(gtx, app, theme)
		}),
	)
}

// renderCommandItem renders a single command entry with source indicator
func renderCommandItem(gtx C, app *appstate.State, theme *material.Theme, cmd *model.CommandEntry, index int, isSelected bool) D {
	// Outer spacing between rows
	return layout.Inset{
		Top:    unit.Dp(1),
		Bottom: unit.Dp(1),
	}.Layout(gtx, func(gtx C) D {
		// Set minimum row height to match tree view
		rowHeight := gtx.Dp(TreeRowHeight)
		gtx.Constraints.Min.Y = rowHeight

		// Use a macro to record operations and get dimensions
		macro := op.Record(gtx.Ops)

		// Inner padding for content (match tree view padding)
		dims := layout.Inset{
			Top:    SpacingXSmall,
			Bottom: SpacingXSmall,
			Left:   SpacingMedium,
			Right:  SpacingMedium,
		}.Layout(gtx, func(gtx C) D {
			return layout.Flex{
				Axis: layout.Vertical,
			}.Layout(gtx,
				// Command text with source icon
				layout.Rigid(func(gtx C) D {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Alignment: layout.Middle,
					}.Layout(gtx,
						// Command text
						layout.Flexed(1.0, func(gtx C) D {
							label := material.Body1(theme, cmd.Command)
							if isSelected {
								label.Color = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}
							}

							return label.Layout(gtx)
						}),
					)
				}),
				// Metadata (frequency and shell name)
				layout.Rigid(func(gtx C) D {
					// Check cache first for zero allocations
					metadata, exists := app.Commands.MetadataCache[cmd]
					if !exists {
						// Build and cache on first render
						metadata = tree.BuildCommandMetadata(cmd.Frequency, cmd.Shell.String())
						app.Commands.MetadataCache[cmd] = metadata
					}

					label := material.Caption(theme, metadata)
					if isSelected {
						label.Color = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorAlpha230}
					} else {
						label.Color.A = ColorGray180 // Semi-transparent
					}

					return layout.Inset{Top: SpacingTiny}.Layout(gtx, label.Layout)
				}),
			)
		})

		call := macro.Stop()

		// draw background with correct dimensions
		isHovered := app.Commands.HoveredIndex == index
		drawSelectionBg(gtx, dims.Size, isSelected, isHovered)

		// Register pointer input area with correct dimensions
		area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
		event.Op(gtx.Ops, cmd)
		area.Pop()

		// Play back the recorded content
		call.Add(gtx.Ops)

		return dims
	})
}

// renderCommandList renders the scrollable list of commands.
//
// THREADING MODEL & SAFETY CONTRACTS:
//
// 1. Parser-synchronized state (protected by app.StoreMu):
//   - app.Commands.DisplayCommands: MUST follow copy-on-write discipline. Always assign a new slice,
//     NEVER mutate in place (no sort, no append to existing slice). This allows shallow copy
//     under RLock to be safe - even if parser assigns a new slice during render, our local
//     copy points to the old immutable slice.
//   - app.LoadError: protected by same lock
//
// 2. UI-thread-only state (NO locking, accessed only from Gio event loop):
//   - SelectedIndex, HoveredIndex, CommandItemHeights, NeedScrollToSel,
//     CommandList.Position, Window
//   - These fields MUST ONLY be accessed from this render function and Gio event handlers.
//   - Background goroutines MUST NOT touch these fields.
func renderCommandList(gtx C, app *appstate.State, theme *material.Theme) D {
	// Shallow copy under RLock is safe due to copy-on-write discipline (see contract above)
	app.StoreMu.RLock()
	commands := app.Commands.DisplayCommands
	loadError := app.LoadError
	app.StoreMu.RUnlock()

	// Initialize or resize height cache if needed
	resizeHeightCache(&app.Commands.ItemHeights, len(commands))

	// Clamp scroll position to valid range
	if len(commands) > 0 && app.Commands.List.Position.First >= len(commands) {
		app.Commands.List.Position.First = len(commands) - 1
		app.Commands.List.Position.Offset = 0
	}

	// Only manipulate scroll state if we're on the Commands tab
	if app.Tabs.Current == model.TabCommands {
		// Clamp selected index if it's out of bounds
		if app.Commands.SelectedIndex >= len(commands) {
			app.Commands.SelectedIndex = -1
			app.NeedScrollToSel = false
		}

		// Set initial selection to last item (newest at bottom) ONLY on first load or tab switch
		// This allows user to deselect intentionally without forcing re-selection
		if app.Commands.NeedInitialSel && app.Commands.SelectedIndex == -1 && len(commands) > 0 {
			app.Commands.SelectedIndex = len(commands) - 1
			app.NeedScrollToSel = true
			app.Commands.NeedInitialSel = false
		}

		// Smart scroll to selected item when explicitly requested
		if app.NeedScrollToSel && app.Commands.SelectedIndex >= 0 && app.Commands.SelectedIndex < len(commands) {
			// Use variable-height scroll calculation with cached heights
			if newFirst, shouldScroll := calculateSmartScrollPositionVariable(gtx, app.Commands.List.Position.First, app.Commands.SelectedIndex, app.Commands.ItemHeights); shouldScroll {
				app.Commands.List.Position.First = newFirst
				app.Commands.List.Position.Offset = 0
			}

			app.NeedScrollToSel = false
		}
	}

	// Handle error state
	if loadError != nil {
		return layout.Center.Layout(gtx, func(gtx C) D {
			label := material.Body1(theme, "Error: "+loadError.Error())
			return layout.Inset{Top: SpacingXXLarge}.Layout(gtx, label.Layout)
		})
	}

	// Handle empty state
	if len(commands) == 0 {
		message := "No commands found"
		if app.CurrentQuery != "" {
			message = fmt.Sprintf("No matches for '%s'", app.CurrentQuery)
		}

		return layout.Center.Layout(gtx, func(gtx C) D {
			label := material.Body1(theme, message)
			return layout.Inset{Top: SpacingXXLarge}.Layout(gtx, label.Layout)
		})
	}

	// Use material.List which properly handles scrolling with pointer events
	return material.List(theme, &app.Commands.List).Layout(gtx, len(commands), func(gtx C, index int) D {
		cmd := commands[index]

		// Process pointer events for this item
		for {
			ev, ok := gtx.Event(pointer.Filter{
				Target: cmd,
				Kinds:  pointer.Enter | pointer.Leave | pointer.Press,
			})
			if !ok {
				break
			}

			if e, ok := ev.(pointer.Event); ok {
				switch e.Kind {
				case pointer.Enter:
					app.Commands.HoveredIndex = index
					app.Window.Invalidate()
				case pointer.Leave:
					if app.Commands.HoveredIndex == index {
						app.Commands.HoveredIndex = -1
						app.Window.Invalidate()
					}
				case pointer.Press:
					app.StoreMu.RLock()

					var cmdToCopy string
					if index >= 0 && index < len(app.Commands.DisplayCommands) {
						cmdToCopy = app.Commands.DisplayCommands[index].Command
					} else if len(app.Commands.DisplayCommands) == 1 {
						cmdToCopy = app.Commands.DisplayCommands[0].Command
					}

					app.StoreMu.RUnlock()

					if cmdToCopy != "" {
						copyTextAndMinimize(gtx, app, cmdToCopy)
					}
				}
			}
		}

		isSelected := app.Commands.SelectedIndex == index

		dims := renderCommandItem(gtx, app, theme, cmd, index, isSelected)

		// Cache the rendered height for smart scrolling
		if index < len(app.Commands.ItemHeights) {
			app.Commands.ItemHeights[index] = dims.Size.Y
		}

		return dims
	})
}
