package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"

	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/model"
)

// statisticsItem represents a unified item (command or prefix) in the statistics view
type statisticsItem struct {
	Text        string
	GlobalIndex int // Unified index (0-9 commands, 10+ prefixes)
	Clickable   *widget.Clickable
	CachedIndex string // Pre-formatted index (e.g., " 1. ")
	CachedCount string // Pre-formatted count (e.g., "    5 uses")
}

// calculateItemBackgroundColor determines the background color for a statistics item
// based on selection and hover state
func calculateItemBackgroundColor(isSelected, isHovered bool) color.NRGBA {
	if isSelected {
		// Selected state - blue background (similar to selected command in Commands tab)
		return color.NRGBA{R: ColorBlueRed, G: ColorBlueGreen, B: ColorBlueBlue, A: ColorAlpha230}
	} else if isHovered {
		// Hover state - slightly lighter gray (only when Statistics tab is active)
		return color.NRGBA{R: ColorDarkGray50, G: ColorDarkGray50, B: ColorDarkGray50, A: ColorWhite}
	}
	// Default state - standard gray
	return color.NRGBA{R: ColorDarkGray40, G: ColorDarkGray40, B: ColorDarkGray40, A: ColorWhite}
}

// renderItemBackground draws a rounded rectangle background with the specified color
func renderItemBackground(gtx C, bgColor color.NRGBA) D {
	return drawRoundedBg(gtx, bgColor, SpacingSmall)
}

// renderItemContent renders the text content of a statistics item using a three-part
// layout: index | text (with fade-out) | count.
func renderItemContent(gtx C, theme *material.Theme, item statisticsItem, isSelected, isHovered bool) D {
	return layout.Inset{
		Top:    SpacingMedium,
		Bottom: SpacingMedium,
		Left:   SpacingMedium,
		Right:  SpacingMedium,
	}.Layout(gtx, func(gtx C) D {
		textColor := color.NRGBA{R: ColorGray220, G: ColorGray220, B: ColorGray220, A: ColorWhite}

		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Baseline,
		}.Layout(gtx,
			// Index label (e.g., " 1. ")
			layout.Rigid(func(gtx C) D {
				label := material.Body1(theme, item.CachedIndex)
				label.Color = textColor
				label.MaxLines = 1

				return label.Layout(gtx)
			}),
			// Command text with fade-out
			layout.Flexed(1.0, func(gtx C) D {
				return renderStatsFadedText(gtx, theme, item.Text, textColor, isSelected, isHovered)
			}),
			// Count label (e.g., "    5 uses")
			layout.Rigid(func(gtx C) D {
				label := material.Body1(theme, item.CachedCount)
				label.Color = textColor
				label.MaxLines = 1

				return label.Layout(gtx)
			}),
		)
	})
}

// renderStatsFadedText renders command text with a fade-out effect when it overflows.
// Uses the same technique as renderFullCommandText in tab_treeview.go.
func renderStatsFadedText(gtx C, theme *material.Theme, text string, textColor color.NRGBA, isSelected, isHovered bool) D {
	maxWidth := gtx.Constraints.Max.X

	// Record the label to measure its natural width
	macro := op.Record(gtx.Ops)
	label := material.Body1(theme, text)
	label.Color = textColor
	label.MaxLines = 1
	dims := label.Layout(gtx)
	call := macro.Stop()

	if dims.Size.X <= maxWidth {
		// Text fits — play back as-is
		call.Add(gtx.Ops)

		return dims
	}

	// Text overflows — clip and overlay fade gradient
	clippedWidth := maxWidth
	clippedHeight := dims.Size.Y

	// Play back recorded ops inside a clip rect
	clipStack := clip.Rect{Max: image.Pt(clippedWidth, clippedHeight)}.Push(gtx.Ops)
	call.Add(gtx.Ops)
	clipStack.Pop()

	// Determine background color for fade (must match item background)
	bgColor := color.NRGBA{R: ColorDarkGray40, G: ColorDarkGray40, B: ColorDarkGray40, A: ColorWhite}
	if isSelected {
		bgColor = color.NRGBA{R: ColorBlueRed, G: ColorBlueGreen, B: ColorBlueBlue, A: ColorWhite}
	} else if isHovered {
		bgColor = color.NRGBA{R: ColorDarkGray50, G: ColorDarkGray50, B: ColorDarkGray50, A: ColorWhite}
	}

	// Overlay fade-out gradient (transparent → background color)
	fadeWidthPx := gtx.Dp(StatsFadeWidth)
	if fadeWidthPx > clippedWidth {
		fadeWidthPx = clippedWidth
	}

	fadeStart := clippedWidth - fadeWidthPx

	renderFadeGradient(gtx.Ops, fadeStart, fadeWidthPx, StatsFadeSteps, clippedHeight, bgColor)

	return D{Size: image.Pt(clippedWidth, clippedHeight)}
}

// renderStatisticsItem renders a single statistics item (command or prefix)
// Returns the dimensions of the rendered item for height caching
func renderStatisticsItem(gtx C, app *appstate.State, theme *material.Theme, item statisticsItem) D {
	for item.Clickable.Clicked(gtx) {
		copyTextAndMinimize(gtx, app, item.Text)
	}

	// hover state
	if item.Clickable.Hovered() && app.Tabs.Current == model.TabTopCommands {
		app.Stats.HoveredIndex = item.GlobalIndex
	} else if app.Stats.HoveredIndex == item.GlobalIndex && !item.Clickable.Hovered() {
		app.Stats.HoveredIndex = -1
	}

	// Determine background color based on selection and hover state
	isSelected := app.Stats.SelectedIndex == item.GlobalIndex
	isHovered := app.Stats.HoveredIndex == item.GlobalIndex && app.Tabs.Current == model.TabTopCommands
	bgColor := calculateItemBackgroundColor(isSelected, isHovered)

	return layout.Inset{Bottom: SpacingXSmall}.Layout(gtx, func(gtx C) D {
		return item.Clickable.Layout(gtx, func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D {
					return renderItemBackground(gtx, bgColor)
				},
				func(gtx C) D {
					return renderItemContent(gtx, theme, item, isSelected, isHovered)
				},
			)
		})
	})
}

// loadCachedStatistics safely reads cached statistics with RLock
func loadCachedStatistics(app *appstate.State) ([]model.RankedEntry, []model.RankedEntry) {
	app.StoreMu.RLock()
	defer app.StoreMu.RUnlock()

	return app.Stats.CachedTopCommands, app.Stats.CachedTopPrefixes
}

// renderLoadingState shows a loading message when statistics are not yet available
func renderLoadingState(gtx C, theme *material.Theme) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		label := material.Body1(theme, TopCommandsLoading)
		return layout.Inset{Top: SpacingXXLarge}.Layout(gtx, label.Layout)
	})
}

// updateStatisticsNavigationState updates counts, arrays, and selection state for keyboard navigation
func updateStatisticsNavigationState(app *appstate.State, topCommands []model.RankedEntry, prefixList []model.RankedEntry) {
	// Store counts for keyboard navigation
	app.Stats.CommandCount = len(topCommands)
	app.Stats.PrefixCount = len(prefixList)

	// Reuse or re-allocate command text slices
	if len(app.Stats.TopCommands) != len(topCommands) {
		app.Stats.TopCommands = make([]string, len(topCommands))
	}

	for i, cmd := range topCommands {
		app.Stats.TopCommands[i] = cmd.Label
	}

	// Reuse or re-allocate prefix text slices
	if len(app.Stats.TopPrefixes) != len(prefixList) {
		app.Stats.TopPrefixes = make([]string, len(prefixList))
	}

	for i, pc := range prefixList {
		app.Stats.TopPrefixes[i] = pc.Label
	}

	// Clamp selection if out of bounds
	totalItems := app.Stats.CommandCount + app.Stats.PrefixCount
	if app.Stats.SelectedIndex >= totalItems {
		app.Stats.SelectedIndex = -1
	}

	// Restore selection by text after window recreation.
	// Search primary list (matching RestoreKind) first, then fallback.
	if app.Stats.RestoreText != "" && totalItems > 0 {
		primary, primaryOffset := topCommands, 0

		fallback, fallbackOffset := prefixList, len(topCommands)
		if app.Stats.RestoreKind != appstate.StatsRestoreCommand {
			primary, primaryOffset, fallback, fallbackOffset = fallback, fallbackOffset, primary, primaryOffset
		}

		if !findStatEntryByLabel(app, primary, primaryOffset) {
			findStatEntryByLabel(app, fallback, fallbackOffset)
		}

		app.Stats.RestoreText = ""
		app.Stats.NeedInitialSel = false
	}

	// Auto-select first item when flag is set (on tab switch)
	if app.Stats.NeedInitialSel && totalItems > 0 {
		app.Stats.SelectedIndex = 0
		app.NeedScrollToSel = true
		app.Stats.NeedInitialSel = false
	}
}

// handleStatisticsScrolling manages scroll-to-selection by directly setting
// the pixel offset. The statistics list is a single-item list (Layout(gtx, 1, ...)),
// so Position.First is always 0 and scrolling is controlled by Position.Offset.
func handleStatisticsScrolling(gtx C, app *appstate.State) {
	totalItems := app.Stats.CommandCount + app.Stats.PrefixCount

	// Only manipulate scroll when on Statistics tab and scroll is needed
	if app.Tabs.Current != model.TabTopCommands || !app.NeedScrollToSel {
		return
	}

	if app.Stats.SelectedIndex < 0 || app.Stats.SelectedIndex >= totalItems {
		app.NeedScrollToSel = false
		return
	}

	// Initialize or resize height cache if needed
	resizeHeightCache(&app.Stats.ItemHeights, totalItems)

	// Sum cached heights of items before the selected one to compute the pixel offset.
	offset := 0

	for i := 0; i < app.Stats.SelectedIndex; i++ {
		h := app.Stats.ItemHeights[i]
		if h <= 0 {
			h = gtx.Dp(TreeRowHeight) + gtx.Dp(TreeRowInsetHeight)
		}

		offset += h
	}

	app.Stats.List.Position.First = 0
	app.Stats.List.Position.Offset = offset

	app.NeedScrollToSel = false
}

// makeStatisticsChild creates a layout child that renders a statistics item and caches its height.
func makeStatisticsChild(app *appstate.State, theme *material.Theme, item statisticsItem, heightIdx int) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		dims := renderStatisticsItem(gtx, app, theme, item)

		// Cache height for this item
		if heightIdx < len(app.Stats.ItemHeights) {
			app.Stats.ItemHeights[heightIdx] = dims.Size.Y
		}

		return dims
	})
}

// renderStatisticsLayout renders the statistics content with top commands and prefixes
func renderStatisticsLayout(gtx C, app *appstate.State, theme *material.Theme, topCommands []model.RankedEntry, prefixList []model.RankedEntry) D {
	return layout.Inset{
		Top:    SpacingXLarge,
		Bottom: SpacingXLarge,
		Left:   SpacingXLarge,
		Right:  SpacingXLarge,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Commands section with title
			layout.Rigid(func(gtx C) D {
				return renderCard(gtx, theme, TopCommandsTop10Title, func(gtx C) D {
					// Use stack-allocated array to avoid heap allocation
					var children [model.TopItemsLimit]layout.FlexChild

					cmds := topCommands
					if len(cmds) > model.TopItemsLimit {
						cmds = cmds[:model.TopItemsLimit]
					}

					for i, cmd := range cmds {
						index := i
						item := statisticsItem{
							Text:        cmd.Label,
							GlobalIndex: index,
							Clickable:   &app.Stats.CommandClickables[index],
							CachedIndex: cmd.CachedIndex,
							CachedCount: cmd.CachedCount,
						}

						children[i] = makeStatisticsChild(app, theme, item, index)
					}

					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children[:len(cmds)]...)
				})
			}),

			// Prefixes section with title
			layout.Rigid(func(gtx C) D {
				return renderCard(gtx, theme, TopCommandsTopPrefixes, func(gtx C) D {
					// Use stack-allocated array to avoid heap allocation
					var children [model.TopItemsLimit]layout.FlexChild

					globalOffset := len(topCommands)

					prefixes := prefixList
					if len(prefixes) > model.TopItemsLimit {
						prefixes = prefixes[:model.TopItemsLimit]
					}

					for i, pc := range prefixes {
						index := i
						globalIndex := globalOffset + index
						item := statisticsItem{
							Text:        pc.Label,
							GlobalIndex: globalIndex,
							Clickable:   &app.Stats.PrefixClickables[index],
							CachedIndex: pc.CachedIndex,
							CachedCount: pc.CachedCount,
						}

						children[i] = makeStatisticsChild(app, theme, item, globalIndex)
					}

					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children[:len(prefixes)]...)
				})
			}),
		)
	})
}

// findStatEntryByLabel searches entries for one matching app.Stats.RestoreText
// and, if found, sets the selection index to offset+i. Returns true on match.
func findStatEntryByLabel(app *appstate.State, entries []model.RankedEntry, offset int) bool {
	for i, entry := range entries {
		if entry.Label == app.Stats.RestoreText {
			app.Stats.SelectedIndex = offset + i
			app.NeedScrollToSel = true

			return true
		}
	}

	return false
}

// renderStatisticsTab renders the statistics view with proper synchronization.
//
// THREADING MODEL & SAFETY CONTRACTS:
//
// 1. Parser-synchronized state (protected by app.StoreMu):
//   - app.Stats.CachedTopCommands, app.Stats.CachedTopPrefixes: Read under RLock by loadCachedStatistics().
//     These slices follow copy-on-write discipline - stats rebuild worker always assigns new
//     slices, never mutates in place. Shallow copy under RLock is safe.
//
// 2. UI-thread-only state (NO locking, accessed only from Gio event loop):
//   - SelectedStatIndex, HoveredStatIndex, StatItemHeights, StatisticsList.Position,
//     StatCommandCount, StatPrefixCount, StatTopCommands, StatTopPrefixes,
//     StatNeedsInitialSel, TopCommandClickables, TopPrefixClickables
//   - These fields MUST ONLY be accessed from this render function and Gio event handlers.
//   - Background goroutines (stats rebuild worker) MUST NOT touch these fields.
func renderStatisticsTab(gtx C, app *appstate.State, theme *material.Theme) D {
	// Load cached statistics (reads under RLock, shallow copy is safe due to copy-on-write)
	topCommands, prefixList := loadCachedStatistics(app)

	// Show loading message if data not yet available
	if topCommands == nil && prefixList == nil {
		return renderLoadingState(gtx, theme)
	}

	// Update navigation state (counts, arrays, selection)
	updateStatisticsNavigationState(app, topCommands, prefixList)

	// Handle scrolling with height caching
	handleStatisticsScrolling(gtx, app)

	// Render the statistics layout
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Flexed(1.0, func(gtx C) D {
			return material.List(theme, &app.Stats.List).Layout(gtx, 1, func(gtx C, _ int) D {
				return renderStatisticsLayout(gtx, app, theme, topCommands, prefixList)
			})
		}),
	)
}
