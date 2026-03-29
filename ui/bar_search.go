package ui

import (
	"image/color"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget/material"

	appstate "github.com/debrief-dev/debrief/app"
)

func renderSearchInput(gtx C, app *appstate.State, theme *material.Theme) D {
	handleSearchKeyboard(gtx, app)

	// Capture text before Layout so we can detect changes after.
	// Do NOT call SearchEditor.Update(gtx) separately — it triggers text
	// shaping (via CaretInfo -> makeValid) with stale layout params, which
	// corrupts the editor's rendering. Let editor.Layout handle Update internally.
	textBefore := app.SearchEditor.Text()

	// Use PassOp to allow scroll events to pass through to the command list
	pass := pointer.PassOp{}.Push(gtx.Ops)
	defer pass.Pop()

	dims := layout.Inset{
		Top:    SpacingMedium,
		Bottom: SpacingMedium,
		Left:   SpacingMedium,
		Right:  SpacingMedium,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			// Hint text
			layout.Rigid(func(gtx C) D {
				hint := material.Caption(theme, tabHint(app.Tabs.Current))
				hint.Color = color.NRGBA{R: ColorGray180, G: ColorGray180, B: ColorGray180, A: ColorWhite}

				return layout.Inset{Bottom: SpacingTiny}.Layout(gtx, hint.Layout)
			}),
			// Search editor
			layout.Rigid(func(gtx C) D {
				editor := material.Editor(theme, &app.SearchEditor, SearchEditorHint)
				// Cap max height to a single line. Without this, the vertical Flex
				// passes its full remaining height as Max.Y, which the editor uses
				// as the viewport height — causing it to expand with each keystroke.
				gtx.Constraints.Max.Y = gtx.Dp(searchEditorMaxHeight)
				dims := editor.Layout(gtx)

				return dims
			}),
		)
	})

	// Detect text changes after Layout (editor.Layout calls Update internally)
	if app.SearchEditor.Text() != textBefore {
		handleSearchInput(app)
	}

	// Request focus on frame 2-4 to ensure UI is ready, or when switching tabs
	app.FrameCount++
	if (app.FrameCount >= 2 && app.FrameCount <= 4) || app.RequestSearchFocus {
		gtx.Execute(key.FocusCmd{Tag: &app.SearchEditor})
		app.RequestSearchFocus = false // Reset flag after focusing
	}

	return dims
}

// handleSearchInput handles changes to the search input.
// Runs search + tree rebuild synchronously on the UI thread (~1-3ms) so that
// renderTreeView (which runs later in the same frame) sees the results immediately.
// This gives 0-frame latency — the user sees results on the same frame they type.
//
//nolint:dupl // selection-save blocks operate on different state (commands vs tree)
func handleSearchInput(app *appstate.State) {
	currentText := app.SearchEditor.Text()

	// Check if query actually changed
	app.StoreMu.RLock()
	queryChanged := currentText != app.CurrentQuery
	app.StoreMu.RUnlock()

	// Only clear selections if the search query actually changed
	// This prevents clearing NeedScrollToSel during tab switches when text hasn't changed
	if queryChanged {
		// Save current selections before clearing them (single lock for consistent snapshot)
		app.StoreMu.RLock()

		if app.Commands.SelectedIndex >= 0 && app.Commands.SelectedIndex < len(app.Commands.DisplayCommands) {
			app.Commands.LastSelectedIndex = app.Commands.SelectedIndex
			app.Commands.LastSelectedCmd = app.Commands.DisplayCommands[app.Commands.SelectedIndex].Command
		}

		if app.Tree.SelectedNode >= 0 && app.Tree.SelectedNode < len(app.Tree.Nodes) {
			app.Tree.LastSelectedNode = app.Tree.SelectedNode
			app.Tree.LastSelectedPath = app.Tree.Nodes[app.Tree.SelectedNode].Path
		}

		app.StoreMu.RUnlock()

		// Reset command selection when user types (return to search mode)
		app.Commands.SelectedIndex = -1 // UI-only state
		app.NeedScrollToSel = false
	}

	// Always update current query
	app.StoreMu.Lock()
	app.CurrentQuery = currentText
	app.StoreMu.Unlock()

	// Only trigger search if query actually changed
	if queryChanged {
		// Run search + tree rebuild synchronously on the UI thread (~1-3ms).
		// Gio's Flex lays out Rigid children (search bar) BEFORE Flexed children
		// (tree view), so results computed here are visible to renderTreeView
		// in the SAME frame — zero latency.
		performSearch(app)

		// Rebuild tree inline for same-frame display.
		rebuildTree(app)

		// Re-pin list to bottom (ScrollToEnd mode). Safe to write Position
		// here because we're on the UI thread.
		app.Commands.List.Position.BeforeEnd = false
	}
}
