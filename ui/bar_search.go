package ui

import (
	"image/color"
	"log"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget/material"
	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/model"
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

// handleSearchInput handles changes to the search input
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
		// Save current selections before clearing them (only if we have a selection)
		if app.Commands.SelectedIndex >= 0 {
			app.StoreMu.RLock()

			if app.Commands.SelectedIndex < len(app.Commands.DisplayCommands) {
				app.Commands.LastSelectedIndex = app.Commands.SelectedIndex
				app.Commands.LastSelectedCmd = app.Commands.DisplayCommands[app.Commands.SelectedIndex].Command
			}

			app.StoreMu.RUnlock()
		}

		if app.Tree.SelectedNode >= 0 {
			app.StoreMu.RLock()

			if app.Tree.SelectedNode < len(app.Tree.Nodes) {
				app.Tree.LastSelectedNode = app.Tree.SelectedNode
				app.Tree.LastSelectedPath = app.Tree.Nodes[app.Tree.SelectedNode].Path
			}

			app.StoreMu.RUnlock()
		}

		// Reset selection when user types (return to search mode)
		app.Commands.SelectedIndex = -1 // UI-only state
		app.NeedScrollToSel = false

		// Clear shared state with lock
		app.StoreMu.Lock()
		app.Tree.SelectedNode = -1
		app.Tree.SelectedNodePath = ""
		app.StoreMu.Unlock()
	}

	// Always update current query
	app.StoreMu.Lock()
	app.CurrentQuery = currentText
	app.StoreMu.Unlock()

	// Only trigger search if query actually changed
	if queryChanged {
		// Execute search immediately
		// Tree will be marked for rebuild when search completes
		executeSearch(app, currentText)
	}
}

// executeSearch performs the actual search
func executeSearch(app *appstate.State, query string) {
	app.StoreMu.Lock()

	executeSearchLocked(app, query)
	// Request tree rebuild after search completes
	RequestTreeRebuild(app)

	app.StoreMu.Unlock()

	app.Window.Invalidate()
}

// executeSearchLocked performs search (must be called with parserMu locked)
func executeSearchLocked(app *appstate.State, query string) {
	// Invalidate height caches when search query changes (different items displayed)
	invalidateHeightCaches(app)

	if query == "" {
		// Empty search: show all loaded commands (respecting shell filter)
		if app.ShellFilter != nil {
			// Apply shell filter
			filtered := make([]*model.CommandEntry, 0, len(app.Commands.LoadedCommands))

			for _, cmd := range app.Commands.LoadedCommands {
				if app.ShellFilter[cmd.Shell] {
					filtered = append(filtered, cmd)
				}
			}

			app.Commands.DisplayCommands = filtered
		} else {
			app.Commands.DisplayCommands = app.Commands.LoadedCommands
		}

		// Scroll to bottom (newest commands at the bottom)
		if len(app.Commands.DisplayCommands) > 0 {
			app.Commands.List.Position.First = len(app.Commands.DisplayCommands) - 1
			app.Commands.List.Position.Offset = 0
		}

		app.SearchMatchingCommands = nil
		app.Commands.NeedInitialSel = true
		app.Tree.NeedInitialSel = true

		log.Printf("Search cleared, showing %d loaded commands", len(app.Commands.DisplayCommands))

		return
	}

	if app.Store == nil {
		return
	}

	// Perform fuzzy search
	log.Printf("Executing search for query: '%s'", query)
	results := app.Store.Search(query)

	// Build matching commands map (for tree rebuild reuse) and DisplayCommands in one pass.
	// results is sorted best-to-worst; DisplayCommands needs worst-to-best (best near search bar).
	// Iterating in reverse gives the correct display order while populating the map.
	matchingMap := make(map[*model.CommandEntry]bool, len(results))

	display := make([]*model.CommandEntry, 0, len(results))
	for i := len(results) - 1; i >= 0; i-- {
		entry := results[i].Entry

		matchingMap[entry] = true
		if app.ShellFilter == nil || app.ShellFilter[entry.Shell] {
			display = append(display, entry)
		}
	}

	app.SearchMatchingCommands = matchingMap
	app.Commands.DisplayCommands = display
	log.Printf("Search completed - found %d matches", len(display))

	// Scroll to bottom (best match at the bottom, near search bar)
	// and auto-select the best match
	if len(app.Commands.DisplayCommands) > 0 {
		app.Commands.List.Position.First = len(app.Commands.DisplayCommands) - 1
		app.Commands.List.Position.Offset = 0
		app.Commands.NeedInitialSel = true
	}

	// Also request initial selection for tree tab after search
	app.Tree.NeedInitialSel = true
}
