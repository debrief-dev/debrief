package ui

import (
	"log"

	"gioui.org/io/key"
	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/model"
)

// handleGlobalKeyboard handles keyboard shortcuts that should work on all tabs
func handleGlobalKeyboard(gtx C, app *appstate.State) {
	// Handle Ctrl+T to cycle between command and tree view
	for {
		ev, ok := gtx.Event(key.Filter{Focus: nil, Name: "T", Required: key.ModShortcut})
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press && e.Modifiers.Contain(key.ModShortcut) {
			switch app.Tabs.Current {
			case model.TabCommands:
				switchToTab(app, model.TabTree)
				log.Println("Switched to tree tab via Ctrl+T")
			case model.TabTree:
				switchToTab(app, model.TabCommands)
				log.Println("Switched to commands tab via Ctrl+T")
			}
		}
	}

	// Handle Ctrl+1/2/3 to switch tabs
	for _, tk := range [...]struct {
		name key.Name
		tab  model.Tab
	}{
		{"1", model.TabCommands},
		{"2", model.TabTree},
		{"3", model.TabTopCommands},
	} {
		for {
			ev, ok := gtx.Event(key.Filter{Focus: nil, Name: tk.name, Required: key.ModShortcut})
			if !ok {
				break
			}

			if e, ok := ev.(key.Event); ok && e.State == key.Press && e.Modifiers.Contain(key.ModShortcut) {
				switchToTab(app, tk.tab)
				log.Printf("Switched to %s tab via Ctrl+%s", tabName(tk.tab), string(tk.name))
			}
		}
	}

	// Global Ctrl+Q handler
	for {
		ev, ok := gtx.Event(key.Filter{Focus: nil, Name: "Q", Required: key.ModShortcut})
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press && e.Modifiers.Contain(key.ModShortcut) {
			log.Println("Global Ctrl+Q handler - exiting application")
			app.QuitFunc()
		}
	}

	// Global ESC handler
	for {
		ev, ok := gtx.Event(key.Filter{Focus: nil, Name: key.NameEscape})
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			log.Println("Global ESC handler - hiding window")

			if app.HideWindowFunc != nil {
				app.HideWindowFunc()
			}
		}
	}
}

// handleStatisticsKeyboard handles keyboard navigation for the Statistics tab.
// Uses Focus: nil to work without requiring a search widget.
func handleStatisticsKeyboard(gtx C, app *appstate.State) {
	if app.Tabs.Current != model.TabTopCommands {
		return
	}

	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: nil, Name: key.NameUpArrow},
			key.Filter{Focus: nil, Name: key.NameDownArrow},
			key.Filter{Focus: nil, Name: key.NamePageUp},
			key.Filter{Focus: nil, Name: key.NamePageDown},
			key.Filter{Focus: nil, Name: key.NameReturn},
			key.Filter{Focus: nil, Name: key.NameEnter},
			key.Filter{Focus: nil, Name: key.NameEscape},
		)
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			totalItems := app.Stats.CommandCount + app.Stats.PrefixCount

			switch e.Name {
			case key.NameReturn, key.NameEnter:
				if app.Stats.SelectedIndex >= 0 && app.Stats.SelectedIndex < totalItems {
					var textToCopy string

					if app.Stats.SelectedIndex < app.Stats.CommandCount {
						if app.Stats.SelectedIndex < len(app.Stats.TopCommands) {
							textToCopy = app.Stats.TopCommands[app.Stats.SelectedIndex]
						}
					} else {
						prefixIndex := app.Stats.SelectedIndex - app.Stats.CommandCount
						if prefixIndex >= 0 && prefixIndex < len(app.Stats.TopPrefixes) {
							textToCopy = app.Stats.TopPrefixes[prefixIndex]
						}
					}

					if textToCopy != "" {
						copyTextAndMinimize(gtx, app, textToCopy)
					}
				}
			case key.NameUpArrow:
				if app.Stats.SelectedIndex == -1 {
					if totalItems > 0 {
						app.Stats.SelectedIndex = totalItems - 1
						app.NeedScrollToSel = true
						app.Window.Invalidate()
					}
				} else if app.Stats.SelectedIndex > 0 {
					app.Stats.SelectedIndex--
					app.NeedScrollToSel = true
					app.Window.Invalidate()
				}
			case key.NameDownArrow:
				if app.Stats.SelectedIndex >= 0 && app.Stats.SelectedIndex < totalItems-1 {
					app.Stats.SelectedIndex++
					app.NeedScrollToSel = true
					app.Window.Invalidate()
				} else if app.Stats.SelectedIndex == totalItems-1 {
					app.Stats.SelectedIndex = -1
					app.Window.Invalidate()
				}
			case key.NamePageUp:
				newIndex := pageJump(app.Stats.SelectedIndex, totalItems, itemsPerPage, true)
				if newIndex != app.Stats.SelectedIndex {
					app.Stats.SelectedIndex = newIndex
					app.NeedScrollToSel = true
					app.Window.Invalidate()
				}
			case key.NamePageDown:
				newIndex := pageJump(app.Stats.SelectedIndex, totalItems, itemsPerPage, false)
				if newIndex != app.Stats.SelectedIndex {
					app.Stats.SelectedIndex = newIndex
					app.NeedScrollToSel = true
					app.Window.Invalidate()
				}
			case key.NameEscape:
				log.Println("Escape in statistics tab - hiding window")

				if app.HideWindowFunc != nil {
					app.HideWindowFunc()
				}
			}
		}
	}
}

// handleSearchKeyboard handles keyboard shortcuts scoped to the search editor.
// All key filters use Focus: &app.SearchEditor so they only fire when the editor has focus.
// This handles: Ctrl+Backspace (delete word), Ctrl+Left/Right (word navigation),
// Ctrl+Up/Down (tree branch navigation), and navigation keys (Up/Down/PageUp/PageDown/Escape/Enter).
func handleSearchKeyboard(gtx C, app *appstate.State) {
	// Ctrl+Backspace to delete word to the left of cursor
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &app.SearchEditor, Name: key.NameDeleteBackward, Required: key.ModShortcut},
		)
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press && e.Modifiers.Contain(key.ModShortcut) {
			text := app.SearchEditor.Text()
			runes := []rune(text)
			_, caretEnd := app.SearchEditor.CaretPos()

			if caretEnd > 0 && caretEnd <= len(runes) {
				wordStart := findPreviousWordBoundary(runes, caretEnd)

				newText := string(runes[:wordStart]) + string(runes[caretEnd:])
				app.SearchEditor.SetText(newText)
				app.SearchEditor.SetCaret(wordStart, wordStart)
				handleSearchInput(app)
				app.Window.Invalidate()
				log.Printf("Ctrl+Backspace in search - deleted word from %d to %d, remaining: %q", wordStart, caretEnd, newText)
			}
		}
	}

	// Ctrl+Left/Right Arrow to skip words
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &app.SearchEditor, Name: key.NameLeftArrow, Required: key.ModShortcut},
			key.Filter{Focus: &app.SearchEditor, Name: key.NameRightArrow, Required: key.ModShortcut},
		)
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press && e.Modifiers.Contain(key.ModShortcut) {
			text := app.SearchEditor.Text()
			runes := []rune(text)
			caretStart, caretEnd := app.SearchEditor.CaretPos()

			log.Printf("Ctrl+Arrow pressed: name=%s, text='%s', caretStart=%d, caretEnd=%d, len(runes)=%d",
				e.Name, text, caretStart, caretEnd, len(runes))

			switch e.Name {
			case key.NameLeftArrow:
				newPos := findPreviousWordBoundary(runes, caretEnd)
				app.SearchEditor.SetCaret(newPos, newPos)
			case key.NameRightArrow:
				newPos := findNextWordBoundary(runes, caretEnd)
				app.SearchEditor.SetCaret(newPos, newPos)
			}

			app.Window.Invalidate()
		}
	}

	// Ctrl+Up for parent navigation in tree view
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &app.SearchEditor, Name: key.NameUpArrow, Required: key.ModShortcut},
		)
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press && e.Modifiers.Contain(key.ModShortcut) {
			if app.Tabs.Current == model.TabTree {
				handleTreeNavigateToPreviousBranch(app)
			}
		}
	}

	// Ctrl+Down for next branch navigation in tree view / jump to end in commands
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &app.SearchEditor, Name: key.NameDownArrow, Required: key.ModShortcut},
		)
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press && e.Modifiers.Contain(key.ModShortcut) {
			switch app.Tabs.Current {
			case model.TabCommands:
				app.StoreMu.RLock()
				count := len(app.Commands.DisplayCommands)
				app.StoreMu.RUnlock()

				if count > 0 {
					app.Commands.SelectedIndex = count - 1
					app.Commands.HoveredIndex = -1
					app.NeedScrollToSel = true
					app.Window.Invalidate()
				}
			case model.TabTree:
				handleTreeNavigateToNextBranch(app)
			}
		}
	}

	// Navigation keys dispatched to per-tab handlers
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &app.SearchEditor, Name: key.NameUpArrow},
			key.Filter{Focus: &app.SearchEditor, Name: key.NameDownArrow},
			key.Filter{Focus: &app.SearchEditor, Name: key.NamePageUp},
			key.Filter{Focus: &app.SearchEditor, Name: key.NamePageDown},
			key.Filter{Focus: &app.SearchEditor, Name: key.NameEscape},
			key.Filter{Focus: &app.SearchEditor, Name: key.NameReturn},
			key.Filter{Focus: &app.SearchEditor, Name: key.NameEnter},
		)
		if !ok {
			break
		}

		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			switch app.Tabs.Current {
			case model.TabCommands:
				handleCommandsKeyNav(gtx, app, e.Name)
			case model.TabTree:
				handleTreeKeyNav(gtx, app, e.Name)
			}
		}
	}
}

// isWordChar returns true if the rune is part of a word (not whitespace)
func isWordChar(r rune) bool {
	return r != ' ' && r != '\t' && r != '\n' && r != '\r'
}

// findPreviousWordBoundary finds the start of the current or previous word from the given position
func findPreviousWordBoundary(runes []rune, pos int) int {
	if pos <= 0 {
		return 0
	}

	i := pos

	for i > 0 && !isWordChar(runes[i-1]) {
		i--
	}

	for i > 0 && isWordChar(runes[i-1]) {
		i--
	}

	log.Printf("findPreviousWordBoundary: pos=%d -> newPos=%d", pos, i)

	return i
}

// findNextWordBoundary finds the start of the next word from the given position
func findNextWordBoundary(runes []rune, pos int) int {
	length := len(runes)
	if pos >= length {
		return length
	}

	i := pos

	for i < length && isWordChar(runes[i]) {
		i++
	}

	for i < length && !isWordChar(runes[i]) {
		i++
	}

	log.Printf("findNextWordBoundary: pos=%d -> newPos=%d", pos, i)

	return i
}

// pageJump computes a new list index after a PageUp or PageDown keystroke.
// current is the selected index (-1 means no selection).
// total is the number of items. pageSize is items to jump.
// up=true moves toward 0; up=false moves toward total-1.
// Returns current unchanged when total == 0.
func pageJump(current, total, pageSize int, up bool) int {
	if total == 0 {
		return current
	}

	var newIndex int

	if current == -1 {
		if up {
			newIndex = total - 1
		} else {
			newIndex = 0
		}
	} else {
		if up {
			newIndex = current - pageSize
		} else {
			newIndex = current + pageSize
		}
	}

	if newIndex < 0 {
		newIndex = 0
	}

	if newIndex >= total {
		newIndex = total - 1
	}

	return newIndex
}

// handleCommandsKeyNav handles a navigation keystroke for the Commands tab.
func handleCommandsKeyNav(gtx C, app *appstate.State, keyName key.Name) {
	switch keyName {
	case key.NameReturn, key.NameEnter:
		app.StoreMu.RLock()

		var cmdToCopy string
		if app.Commands.SelectedIndex >= 0 && app.Commands.SelectedIndex < len(app.Commands.DisplayCommands) {
			cmdToCopy = app.Commands.DisplayCommands[app.Commands.SelectedIndex].Command
		} else if len(app.Commands.DisplayCommands) == 1 {
			cmdToCopy = app.Commands.DisplayCommands[0].Command
		}

		app.StoreMu.RUnlock()

		if cmdToCopy != "" {
			copyTextAndMinimize(gtx, app, cmdToCopy)
		}
	case key.NameUpArrow:
		if app.Commands.SelectedIndex == -1 {
			app.StoreMu.RLock()
			count := len(app.Commands.DisplayCommands)
			app.StoreMu.RUnlock()

			if count > 0 {
				app.Commands.SelectedIndex = count - 1
				app.Commands.HoveredIndex = -1
				app.NeedScrollToSel = true
				app.Window.Invalidate()
			}
		} else if app.Commands.SelectedIndex > 0 {
			app.StoreMu.RLock()
			count := len(app.Commands.DisplayCommands)
			app.StoreMu.RUnlock()

			if app.Commands.SelectedIndex < count {
				app.Commands.SelectedIndex--
				app.Commands.HoveredIndex = -1
				app.NeedScrollToSel = true
				app.Window.Invalidate()
			}
		}
	case key.NameDownArrow:
		app.StoreMu.RLock()
		count := len(app.Commands.DisplayCommands)
		app.StoreMu.RUnlock()

		if app.Commands.SelectedIndex >= 0 && app.Commands.SelectedIndex < count-1 {
			app.Commands.SelectedIndex++
			app.Commands.HoveredIndex = -1
			app.NeedScrollToSel = true
			app.Window.Invalidate()
		}
		// At last item: stay selected (don't deselect)
	case key.NamePageUp, key.NamePageDown:
		app.StoreMu.RLock()
		count := len(app.Commands.DisplayCommands)
		app.StoreMu.RUnlock()

		newIndex := pageJump(app.Commands.SelectedIndex, count, itemsPerPage, keyName == key.NamePageUp)
		if newIndex != app.Commands.SelectedIndex {
			app.Commands.SelectedIndex = newIndex
			app.Commands.HoveredIndex = -1
			app.NeedScrollToSel = true
			app.Window.Invalidate()
		}
	case key.NameEscape:
		log.Println("Escape in commands tab - hiding window")

		if app.HideWindowFunc != nil {
			app.HideWindowFunc()
		}
	}
}

// handleTreeKeyNav handles a navigation keystroke for the Tree tab.
// Acquires app.StoreMu as needed for tree-state mutations.
func handleTreeKeyNav(gtx C, app *appstate.State, keyName key.Name) {
	switch keyName {
	case key.NameReturn, key.NameEnter:
		app.StoreMu.RLock()

		if app.Tree.SelectedNode >= 0 && app.Tree.SelectedNode < len(app.Tree.Nodes) {
			node := app.Tree.Nodes[app.Tree.SelectedNode]
			app.StoreMu.RUnlock()
			copyTextAndMinimize(gtx, app, node.Path)
		} else {
			app.StoreMu.RUnlock()
		}
	case key.NameUpArrow:
		app.StoreMu.RLock()
		selectedTreeNode := app.Tree.SelectedNode
		count := len(app.Tree.Nodes)
		app.StoreMu.RUnlock()

		if selectedTreeNode == -1 {
			if count > 0 {
				app.StoreMu.Lock()
				app.Tree.SelectedNode = count - 1
				app.Tree.SelectedNodePath = ""
				app.StoreMu.Unlock()

				app.Tree.HoveredNode = -1
				app.NeedScrollToSel = true
				app.Window.Invalidate()
			}
		} else if selectedTreeNode > 0 {
			app.StoreMu.Lock()
			app.Tree.SelectedNode--
			app.Tree.SelectedNodePath = ""
			app.StoreMu.Unlock()

			app.Tree.HoveredNode = -1
			app.NeedScrollToSel = true
			app.Window.Invalidate()
		}
	case key.NameDownArrow:
		app.StoreMu.RLock()
		selectedTreeNode := app.Tree.SelectedNode
		count := len(app.Tree.Nodes)
		app.StoreMu.RUnlock()

		if selectedTreeNode >= 0 && selectedTreeNode < count-1 {
			app.StoreMu.Lock()
			app.Tree.SelectedNode++
			app.Tree.SelectedNodePath = ""
			app.StoreMu.Unlock()

			app.Tree.HoveredNode = -1
			app.NeedScrollToSel = true
			app.Window.Invalidate()
		}
	case key.NamePageUp, key.NamePageDown:
		app.StoreMu.RLock()
		selectedTreeNode := app.Tree.SelectedNode
		count := len(app.Tree.Nodes)
		app.StoreMu.RUnlock()

		newIndex := pageJump(selectedTreeNode, count, nodesPerPage, keyName == key.NamePageUp)
		if newIndex != selectedTreeNode {
			app.StoreMu.Lock()
			app.Tree.SelectedNode = newIndex
			app.Tree.SelectedNodePath = ""
			app.StoreMu.Unlock()

			app.Tree.HoveredNode = -1
			app.NeedScrollToSel = true
			app.Window.Invalidate()
		}
	case key.NameEscape:
		log.Println("Escape in tree tab - hiding window")

		if app.HideWindowFunc != nil {
			app.HideWindowFunc()
		}
	}
}
