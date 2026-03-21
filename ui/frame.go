package ui

import (
	"gioui.org/layout"
	"gioui.org/widget/material"

	appstate "github.com/debrief-dev/debrief/app"
	"github.com/debrief-dev/debrief/data/model"
)

// invalidateHeightCaches clears the height caches to force recalculation.
// Call this when content changes (search, filter, resize) that affects item heights.
func invalidateHeightCaches(app *appstate.State) {
	if app.Commands.ItemHeights != nil {
		app.Commands.ItemHeights = app.Commands.ItemHeights[:0]
	}

	if app.Tree.ItemHeights != nil {
		app.Tree.ItemHeights = app.Tree.ItemHeights[:0]
	}
}

func RenderFrame(gtx C, app *appstate.State, theme *material.Theme) D {
	// Detect window resize and invalidate height caches (text reflows change item heights)
	currentSize := gtx.Constraints.Max
	if app.LastWindowSize.X != currentSize.X || app.LastWindowSize.Y != currentSize.Y {
		if app.LastWindowSize.X != 0 || app.LastWindowSize.Y != 0 {
			invalidateHeightCaches(app)
		}

		app.LastWindowSize = currentSize
	}

	// Handle global keyboard shortcuts first (works on all tabs)
	handleGlobalKeyboard(gtx, app)

	// Handle Statistics tab keyboard navigation (when on Statistics tab)
	handleStatisticsKeyboard(gtx, app)

	// Main layout with tabs
	dims := layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Tab bar at top (fixed)
		layout.Rigid(func(gtx C) D {
			return renderTabBar(gtx, app, theme)
		}),

		// Tab content (takes remaining space)
		layout.Flexed(1.0, func(gtx C) D {
			switch app.Tabs.Current {
			case model.TabCommands:
				return renderCommandsTab(gtx, app, theme)
			case model.TabTree:
				return renderTreeTab(gtx, app, theme)
			case model.TabTopCommands:
				return renderStatisticsTab(gtx, app, theme)
			case model.TabSettings:
				return renderSettingsTab(gtx, app, theme)
			default:
				return renderCommandsTab(gtx, app, theme)
			}
		}),
	)

	return dims
}
