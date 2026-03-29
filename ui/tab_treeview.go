package ui

import (
	"image"
	"image/color"
	"log"
	"sync"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"

	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/model"
)

// treeRestoreContextRows is the number of rows shown above a restored
// selection so the user sees surrounding context.
const treeRestoreContextRows = 3

// treeNavigationResult holds the result of a tree navigation search
type treeNavigationResult struct {
	foundIndex int
	foundPath  string
}

var (
	expandMoreIcon *widget.Icon
	iconsOnce      sync.Once
)

// initializeIcons lazily initializes the tree view icon
func initializeIcons() {
	iconsOnce.Do(func() {
		var err error

		expandMoreIcon, err = widget.NewIcon(icons.NavigationExpandMore)
		if err != nil {
			log.Printf("Failed to create expand more icon: %v", err)
		}
	})
}

// renderTreeTab renders the new tree view
//
//nolint:dupl // label+inset patterns are structurally similar but have different content
func renderTreeTab(gtx C, app *appstate.State, theme *material.Theme) D {
	// Keyboard navigation is now handled in RenderSearchInput based on current tab
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Tree view (takes remaining space)
		layout.Flexed(1.0, func(gtx C) D {
			return renderTreeView(gtx, app, theme)
		}),
		// Shell filter bar (shared with commands tab)
		layout.Rigid(func(gtx C) D {
			return renderShellFilterBar(gtx, app, theme)
		}),
		// Search input (shared with commands tab)
		layout.Rigid(func(gtx C) D {
			return renderSearchInput(gtx, app, theme)
		}),
	)
}

// renderTreeView renders the hierarchical tree view
// renderTreeView renders the tree view with proper synchronization.
//
// THREADING MODEL & SAFETY CONTRACTS:
//
// 1. Parser-synchronized state (protected by app.StoreMu):
//   - app.Tree.Nodes: MUST follow copy-on-write discipline. Always assign a new slice,
//     NEVER mutate in place. This allows shallow copy under RLock to be safe - even if
//     rebuild worker assigns a new slice during render, our local copy points to the old
//     immutable slice.
//   - app.Tree.SelectedNode, app.Tree.SelectedNodePath: Protected by StoreMu for atomic
//     updates with TreeNodes to prevent TOCTOU races during tree rebuilds.
//   - app.NeedScrollToSel: Snapshot under RLock for consistency.
//
// 2. UI-thread-only state (NO locking, accessed only from Gio event loop):
//   - TreeNodeHeights, TreeList.Position, HoveredTreeNode, SuppressTreeHover,
//     LastSelectedTreeNode, LastSelectedTreePath
//   - These fields MUST ONLY be accessed from this render function and Gio event handlers.
//   - Background goroutines (tree rebuild worker) MUST NOT touch these fields.
func renderTreeView(gtx C, app *appstate.State, theme *material.Theme) D {
	initializeIcons()
	// Take a consistent snapshot of all tree-related state with RLock
	// This prevents TOCTOU races where TreeNodes changes between accesses
	// Shallow copy of nodes is safe due to copy-on-write discipline (see contract above)
	app.StoreMu.RLock()
	nodes := app.Tree.Nodes
	selectedTreeNode := app.Tree.SelectedNode
	selectedTreeNodePath := app.Tree.SelectedNodePath
	needScrollToSel := app.NeedScrollToSel
	app.StoreMu.RUnlock()

	// Initialize or resize height cache if needed (UI-only state, no lock needed)
	resizeHeightCache(&app.Tree.ItemHeights, len(nodes))

	// Clamp scroll position to valid range to prevent out-of-bounds rendering (UI-only state)
	if len(nodes) > 0 && app.Tree.List.Position.First >= len(nodes) {
		app.Tree.List.Position.First = len(nodes) - 1
		app.Tree.List.Position.Offset = 0
	}

	// Restore selection by path after rebuild (atomic operation).
	// Skip when nodes is empty — the tree rebuild is async and may not have
	// delivered data yet. Clearing the path on an empty list would lose the
	// saved selection permanently.
	if selectedTreeNodePath != "" && len(nodes) > 0 {
		// Search in snapshot
		foundIndex := -1

		for i, node := range nodes {
			if node.Path == selectedTreeNodePath {
				foundIndex = i
				break
			}
		}

		// Atomically update selection and clear path under single Lock
		app.StoreMu.Lock()

		if foundIndex >= 0 && foundIndex < len(app.Tree.Nodes) {
			// Re-validate path after acquiring lock (tree might have been rebuilt)
			if app.Tree.Nodes[foundIndex].Path == selectedTreeNodePath {
				app.Tree.SelectedNode = foundIndex
				app.Tree.SelectedNodePath = ""

				// Directly position list to show the restored item.
				// Height caches are empty at this point (tree just rebuilt),
				// so the deferred NeedScrollToSel calculation is unreliable.
				// Position the item a few rows from the top for context.
				first := max(foundIndex-treeRestoreContextRows, 0)

				app.Tree.List.Position.First = first
				app.Tree.List.Position.Offset = 0
			}
		} else if foundIndex < 0 {
			// Path genuinely not found in current nodes — clear it.
			// When foundIndex >= 0 but >= len(app.Tree.Nodes), the tree
			// was rebuilt concurrently; keep the path for retry next frame.
			app.Tree.SelectedNodePath = ""
		}

		app.StoreMu.Unlock()

		// Update local variable for rendering
		selectedTreeNode = app.Tree.SelectedNode
	}

	// Clamp selected tree node if it's out of bounds
	if selectedTreeNode >= len(nodes) {
		app.StoreMu.Lock()
		// Re-check bounds after acquiring lock
		if app.Tree.SelectedNode >= len(app.Tree.Nodes) {
			app.Tree.SelectedNode = -1
			app.Tree.SelectedNodePath = ""
		}

		selectedTreeNode = app.Tree.SelectedNode
		app.StoreMu.Unlock()

		app.NeedScrollToSel = false
	}

	// Auto-select tree node matching the search query.
	// Best match index is pre-computed by rebuildTreeLocked to avoid
	// O(n) scan on the UI thread. Falls back to last item when no search is active.
	//
	// Deferred while a tree rebuild is pending because the node list may be stale.
	if app.Tree.NeedInitialSel && len(nodes) > 0 &&
		!app.Tree.NeedsRebuild.Load() {
		app.StoreMu.RLock()
		bestMatch := app.Tree.BestMatchIndex
		app.StoreMu.RUnlock()

		targetIndex := len(nodes) - 1 // fallback: last item, like Commands tab
		if bestMatch >= 0 && bestMatch < len(nodes) {
			targetIndex = bestMatch
		}

		app.StoreMu.Lock()
		if targetIndex < len(app.Tree.Nodes) {
			app.Tree.SelectedNode = targetIndex
			app.Tree.SelectedNodePath = ""
		}

		selectedTreeNode = app.Tree.SelectedNode
		app.StoreMu.Unlock()

		needScrollToSel = true // Update local var so scroll runs in the SAME frame
		app.NeedScrollToSel = true

		app.Tree.NeedInitialSel = false
	} else if app.Tree.NeedInitialSel {
		if len(nodes) == 0 {
			app.Tree.NeedInitialSel = false
		}
	}

	// Handle empty state
	if len(nodes) == 0 {
		return layout.Center.Layout(gtx, func(gtx C) D {
			label := material.Body1(theme, "No commands available")
			return layout.Inset{Top: SpacingXXLarge}.Layout(gtx, label.Layout)
		})
	}

	// Smart scroll-to-selection for keyboard navigation (UI-only state)
	if needScrollToSel && selectedTreeNode >= 0 && selectedTreeNode < len(nodes) {
		// Use variable-height scroll calculation with cached heights
		if newFirst, shouldScroll := calculateSmartScrollPositionVariable(gtx, app.Tree.List.Position.First, selectedTreeNode, app.Tree.ItemHeights); shouldScroll {
			app.Tree.List.Position.First = newFirst
			app.Tree.List.Position.Offset = 0
		}

		app.NeedScrollToSel = false
	}

	// Render hint and tree list in vertical layout
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Tree list
		layout.Flexed(1.0, func(gtx C) D {
			return material.List(theme, &app.Tree.List).Layout(gtx, len(nodes), func(gtx C, index int) D {
				node := nodes[index]
				isSelected := selectedTreeNode == index
				isHovered := app.Tree.HoveredNode == index // HoveredTreeNode is UI-only, no lock needed

				// Handle pointer events
				handleTreeNodePointerEvents(gtx, app, node, index)

				dims := renderTreeNode(gtx, theme, node, isSelected, isHovered)

				// Cache the rendered height for smart scrolling (UI-only state)
				if index < len(app.Tree.ItemHeights) {
					app.Tree.ItemHeights[index] = dims.Size.Y
				}

				return dims
			})
		}),
	)
}

// renderFullCommandText renders the full command path with gray prefix and black current word.
// Long prefixes are truncated with a fade-out effect to ensure the current word remains visible.
func renderFullCommandText(gtx C, theme *material.Theme, pathPrefix, currentWord string, isSelected bool) D {
	// Render using horizontal flex with two labels
	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Baseline,
	}.Layout(gtx,
		// Gray prefix (if exists) - includes trailing space
		layout.Rigid(func(gtx C) D {
			if pathPrefix == "" {
				return D{}
			}

			// Cap prefix width so the current word always has space
			maxPrefixWidth := gtx.Constraints.Max.X * TreePrefixMaxWidthPercent / TreePrefixPercentBase

			// Measure natural prefix width using op.Record
			macro := op.Record(gtx.Ops)
			label := material.Body1(theme, pathPrefix)

			label.MaxLines = 1
			if isSelected {
				label.Color = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorAlpha230}
			} else {
				label.Color = color.NRGBA{R: ColorGray180, G: ColorGray180, B: ColorGray180, A: ColorWhite}
			}

			dims := label.Layout(gtx)
			call := macro.Stop()

			if dims.Size.X <= maxPrefixWidth {
				// Prefix fits — play back as-is, no fade needed
				call.Add(gtx.Ops)
				return dims
			}

			// Prefix overflows — clip recorded output and overlay fade
			clippedWidth := maxPrefixWidth
			clippedHeight := dims.Size.Y

			// Play back the recorded ops inside a clip rect
			clipStack := clip.Rect{Max: image.Pt(clippedWidth, clippedHeight)}.Push(gtx.Ops)
			call.Add(gtx.Ops)
			clipStack.Pop()

			// Overlay fade-out gradient (transparent → background color)
			bgColor := theme.Bg
			if isSelected {
				bgColor = color.NRGBA{R: ColorBlueRed, G: ColorBlueGreen, B: ColorBlueBlue, A: ColorWhite}
			}

			fadeWidthPx := min(gtx.Dp(TreePrefixFadeWidth), clippedWidth)

			fadeStart := clippedWidth - fadeWidthPx

			renderFadeGradient(gtx.Ops, fadeStart, fadeWidthPx, TreePrefixFadeSteps, clippedHeight, bgColor)

			return D{Size: image.Pt(clippedWidth, clippedHeight)}
		}),
		// Black current word
		layout.Flexed(1.0, func(gtx C) D {
			label := material.Body1(theme, currentWord)

			label.MaxLines = 0
			if isSelected {
				label.Color = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}
			}

			return label.Layout(gtx)
		}),
	)
}

// renderTreeNode renders a single tree node
func renderTreeNode(gtx C, theme *material.Theme, node *model.TreeDisplayNode, isSelected, isHovered bool) D {
	return layout.Inset{
		Top:    unit.Dp(1),
		Bottom: unit.Dp(1),
	}.Layout(gtx, func(gtx C) D {
		// Set minimum row height, allow expansion for multiline text
		rowHeight := gtx.Dp(TreeRowHeight)
		gtx.Constraints.Min.Y = rowHeight

		macro := op.Record(gtx.Ops)

		dims := layout.Inset{
			Top:    SpacingXSmall,
			Bottom: SpacingXSmall,
			Left:   unit.Dp(TreeIndentBase + float32(node.Depth)*TreeIndentMultiplier), // Indent based on depth
			Right:  SpacingMedium,
		}.Layout(gtx, func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				// Tree structure icon (arrow down for nodes with children)
				layout.Rigid(func(gtx C) D {
					// Fixed width for icon area
					iconWidth := gtx.Dp(TreeIconWidth)
					gtx.Constraints.Min.X = iconWidth
					gtx.Constraints.Max.X = iconWidth

					if !node.HasChildren {
						return D{Size: image.Pt(iconWidth, gtx.Constraints.Min.Y)}
					}

					iconWidget := expandMoreIcon

					if iconWidget == nil {
						label := material.Body1(theme, "▼")
						if isSelected {
							label.Color = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}
						}

						return layout.Inset{Right: SpacingXSmall}.Layout(gtx, label.Layout)
					}

					iconColor := theme.Fg
					if isSelected {
						iconColor = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}
					}

					return layout.Inset{
						Right: SpacingXSmall,
					}.Layout(gtx, func(gtx C) D {
						// Set icon size to be square
						iconSize := gtx.Dp(TreeIconSize)
						gtx.Constraints.Min = image.Pt(iconSize, iconSize)
						gtx.Constraints.Max = image.Pt(iconSize, iconSize)

						return iconWidget.Layout(gtx, iconColor)
					})
				}),
				// Node text and metadata (vertical layout)
				layout.Flexed(1.0, func(gtx C) D {
					return layout.Flex{
						Axis: layout.Vertical,
					}.Layout(gtx,
						// Node text
						layout.Rigid(func(gtx C) D {
							// All nodes show full path with gray prefix and black current word
							// Use cached PathPrefixWithSpace to avoid string allocations at render time
							return renderFullCommandText(gtx, theme, node.PathPrefixWithSpace, node.Node.Word, isSelected)
						}),
						// Metadata (command count or usage info)
						layout.Rigid(func(gtx C) D {
							metadata := node.CachedMetadata

							label := material.Caption(theme, metadata)
							if isSelected {
								label.Color = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorAlpha230}
							} else {
								label.Color.A = ColorGray180 // Semi-transparent
							}

							return layout.Inset{Top: SpacingTiny}.Layout(gtx, label.Layout)
						}),
					)
				}),
			)
		})

		call := macro.Stop()

		// Draw background
		drawSelectionBg(gtx, dims.Size, isSelected, isHovered)

		// Register pointer area
		area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
		event.Op(gtx.Ops, node)
		area.Pop()

		// Play back content
		call.Add(gtx.Ops)

		return dims
	})
}

// handleTreeNodePointerEvents processes clicks and hovers for tree nodes
func handleTreeNodePointerEvents(gtx C, app *appstate.State, node *model.TreeDisplayNode, index int) {
	needsInvalidate := false

	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: node,
			Kinds:  pointer.Enter | pointer.Leave | pointer.Press | pointer.Move,
		})
		if !ok {
			break
		}

		if e, ok := ev.(pointer.Event); ok {
			switch e.Kind {
			case pointer.Move, pointer.Enter:
				// Mouse moved - clear suppress flag without applying hover
				if app.Tree.SuppressHover {
					app.Tree.SuppressHover = false
					needsInvalidate = true
				} else if app.Tree.HoveredNode != index {
					app.Tree.HoveredNode = index
					needsInvalidate = true
				}
			case pointer.Leave:
				if app.Tree.HoveredNode == index {
					app.Tree.HoveredNode = -1
					needsInvalidate = true
				}
			case pointer.Press:
				handleTreeNodeClick(gtx, app, index)
			}
		}
	}

	if needsInvalidate {
		app.Window.Invalidate()
	}
}

// handleTreeNodeClick handles clicks on tree nodes.
// Re-reads the current node at the given index to get the latest state.
func handleTreeNodeClick(gtx C, app *appstate.State, index int) {
	app.StoreMu.RLock()

	if index < 0 || index >= len(app.Tree.Nodes) {
		app.StoreMu.RUnlock()
		return
	}

	node := app.Tree.Nodes[index]
	app.StoreMu.RUnlock()

	// Copy the node path and minimize (matches Enter key behavior)
	if node.Path != "" {
		copyTextAndMinimize(gtx, app, node.Path)
	}
}

// navigateTreeWithSearch performs tree navigation using a custom search function
// searchFn takes (nodes, selectedIndex) and returns search result
func navigateTreeWithSearch(app *appstate.State, searchFn func(nodes []*model.TreeDisplayNode, selectedIndex int) treeNavigationResult) {
	// Snapshot with generation counter (copy-on-write: nodes slice is immutable after snapshot)
	app.StoreMu.RLock()
	selectedTreeNode := app.Tree.SelectedNode
	nodes := app.Tree.Nodes
	generation := app.Tree.NodesGeneration
	app.StoreMu.RUnlock()

	if selectedTreeNode < 0 || selectedTreeNode >= len(nodes) {
		return
	}

	result := searchFn(nodes, selectedTreeNode)

	if result.foundIndex >= 0 {
		app.StoreMu.Lock()

		// Strong validation: check generation + bounds + path
		if app.Tree.NodesGeneration == generation &&
			result.foundIndex < len(app.Tree.Nodes) &&
			app.Tree.Nodes[result.foundIndex].Path == result.foundPath {
			app.Tree.SelectedNode = result.foundIndex
			app.Tree.SelectedNodePath = ""
			app.StoreMu.Unlock()

			app.Tree.HoveredNode = -1
			app.NeedScrollToSel = true
			app.Window.Invalidate()
		} else {
			// Tree was rebuilt, use path-based restoration
			app.Tree.SelectedNodePath = result.foundPath
			app.StoreMu.Unlock()

			app.Tree.HoveredNode = -1
			app.Window.Invalidate()
		}
	}
}

// handleTreeNavigateToPreviousBranch navigates to the previous branch node (node with children)
func handleTreeNavigateToPreviousBranch(app *appstate.State) {
	navigateTreeWithSearch(app, func(nodes []*model.TreeDisplayNode, selectedIndex int) treeNavigationResult {
		// Search backward for previous node with children
		for i := selectedIndex - 1; i >= 0; i-- {
			node := nodes[i]
			if node.HasChildren {
				return treeNavigationResult{
					foundIndex: i,
					foundPath:  node.Path,
				}
			}
		}

		return treeNavigationResult{
			foundIndex: -1,
		}
	})
}

// handleTreeNavigateToNextBranch navigates to the next branch node (node with children)
func handleTreeNavigateToNextBranch(app *appstate.State) {
	navigateTreeWithSearch(app, func(nodes []*model.TreeDisplayNode, selectedIndex int) treeNavigationResult {
		// Search forward for next node with children
		for i := selectedIndex + 1; i < len(nodes); i++ {
			node := nodes[i]
			if node.HasChildren {
				return treeNavigationResult{
					foundIndex: i,
					foundPath:  node.Path,
				}
			}
		}

		return treeNavigationResult{
			foundIndex: -1,
		}
	})
}
