package ui

import (
	"image/color"
	"log"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/model"
	"github.com/debrief-dev/debrief/shell"
)

// renderShellBadge renders a single clickable shell filter badge with click handling
func renderShellBadge(gtx C, app *appstate.State, theme *material.Theme, name string, isSelected bool, onClick func()) D {
	badgeWidget, exists := app.ShellBadges[name]
	if !exists {
		badgeWidget = new(widget.Clickable)
		app.ShellBadges[name] = badgeWidget
	}

	for badgeWidget.Clicked(gtx) {
		onClick()
	}

	return renderBadgeUI(gtx, theme, name, isSelected, badgeWidget)
}

// renderBadgeUI renders just the badge UI (without click handling)
func renderBadgeUI(gtx C, theme *material.Theme, name string, isSelected bool, badgeWidget *widget.Clickable) D {
	return layout.Inset{Right: SpacingSmall}.Layout(gtx, func(gtx C) D {
		return material.Clickable(gtx, badgeWidget, func(gtx C) D {
			// Badge background color
			bgColor := color.NRGBA{R: ColorDarkGray60, G: ColorDarkGray60, B: ColorDarkGray60, A: ColorGrayAlpha}
			if isSelected {
				bgColor = color.NRGBA{R: ColorBlueRed, G: ColorBlueGreen, B: ColorBlueBlue, A: ColorBlueAlpha}
			}

			return layout.Background{}.Layout(gtx,
				func(gtx C) D {
					return drawRoundedBg(gtx, bgColor, BorderRadius)
				},
				func(gtx C) D {
					return layout.Inset{
						Top:    SpacingXSmall,
						Bottom: SpacingXSmall,
						Left:   SpacingMedium,
						Right:  SpacingMedium,
					}.Layout(gtx, func(gtx C) D {
						label := material.Caption(theme, name)
						if isSelected {
							label.Color = color.NRGBA{R: ColorWhite, G: ColorWhite, B: ColorWhite, A: ColorWhite}
						}

						return label.Layout(gtx)
					})
				},
			)
		})
	})
}

// applyShellFilterLocked filters displayed commands based on selected shells (must be called with StoreMu locked)
func applyShellFilterLocked(app *appstate.State) {
	// Request tree rebuild when filter changes
	RequestTreeRebuild(app)
	requestStatsRebuild(app)

	// Invalidate height caches when shell filter changes (different items displayed)
	invalidateHeightCaches(app)

	if app.ShellFilter == nil {
		app.Commands.DisplayCommands = app.Commands.LoadedCommands

		log.Println("Shell filter cleared - showing all shells")
		app.MarkDirty()

		return
	}

	filtered := make([]*model.CommandEntry, 0, len(app.Commands.LoadedCommands))
	for _, cmd := range app.Commands.LoadedCommands {
		if app.ShellFilter[cmd.Shell] {
			filtered = append(filtered, cmd)
		}
	}

	app.Commands.DisplayCommands = filtered
	log.Printf("Shell filter applied - showing %d of %d commands",
		len(filtered), len(app.Commands.LoadedCommands))
	app.MarkDirty()
}

// renderShellBadgeList renders all individual shell filter badges
func renderShellBadgeList(gtx C, app *appstate.State, theme *material.Theme, sortedSources []*shell.ShellMetadata, currentFilter map[model.Shell]bool) D {
	// Use stack-allocated array to avoid heap allocation
	var badges [MaxShellBadges]layout.FlexChild

	count := len(sortedSources)
	if count > MaxShellBadges {
		count = MaxShellBadges
	}

	for i := 0; i < count; i++ {
		source := sortedSources[i]
		src := source // Capture for closure
		isSelected := currentFilter != nil && currentFilter[src.Type]

		badges[i] = layout.Rigid(func(gtx C) D {
			return renderShellBadge(gtx, app, theme, src.Type.String(), isSelected, func() {
				// Must lock before modifying ShellFilter (shared with background worker)
				app.StoreMu.Lock()

				// Toggle this shell in filter
				if app.ShellFilter == nil {
					app.ShellFilter = make(map[model.Shell]bool)
				}

				if app.ShellFilter[src.Type] {
					delete(app.ShellFilter, src.Type)

					if len(app.ShellFilter) == 0 {
						app.ShellFilter = nil
					}
				} else {
					app.ShellFilter[src.Type] = true
				}

				applyShellFilterLocked(app)
				app.StoreMu.Unlock()
			})
		})
	}

	return layout.Flex{
		Axis:    layout.Horizontal,
		Spacing: layout.SpaceEnd,
	}.Layout(gtx, badges[:count]...)
}

// renderShellFilterBar renders the shell filter badges
func renderShellFilterBar(gtx C, app *appstate.State, theme *material.Theme) D {
	enabledSources := app.SourceManager.Enabled()
	if len(enabledSources) <= 1 {
		return D{}
	}

	// Snapshot filter state once for the entire frame (consistent view, avoids per-badge locking)
	app.StoreMu.RLock()
	currentFilter := app.ShellFilter
	app.StoreMu.RUnlock()

	isAllSelected := currentFilter == nil

	return layout.Inset{
		Top:    SpacingXSmall,
		Bottom: SpacingXSmall,
		Left:   SpacingMedium,
		Right:  SpacingMedium,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Spacing:   layout.SpaceEnd,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// "Shell:" label
			layout.Rigid(func(gtx C) D {
				label := material.Caption(theme, "Shell:")
				label.Color.A = ColorGray180

				return layout.Inset{Right: SpacingMedium}.Layout(gtx, label.Layout)
			}),
			// "All" badge
			layout.Rigid(func(gtx C) D {
				// Must lock before modifying ShellFilter (shared with background worker)
				for app.AllBadge.Clicked(gtx) {
					app.StoreMu.Lock()
					app.ShellFilter = nil
					applyShellFilterLocked(app)
					app.StoreMu.Unlock()
				}

				return renderBadgeUI(gtx, theme, "All", isAllSelected, &app.AllBadge)
			}),
			// Individual shell badges
			layout.Rigid(func(gtx C) D {
				return renderShellBadgeList(gtx, app, theme, enabledSources, currentFilter)
			}),
		)
	})
}
