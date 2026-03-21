package ui

import (
	"gioui.org/unit"

	"github.com/debrief-dev/debrief/data/model"
	"github.com/debrief-dev/debrief/infra/hotkey"
	"github.com/debrief-dev/debrief/infra/platform"
)

// -----
// UI STRINGS

const WindowTitle = "Debrief"

// Tray menu Strings
const (
	TrayShowWindowTitle   = "Show Debrief"
	TrayShowWindowTooltip = "Show window"

	TrayHideWindowTitle   = "Hide Debrief"
	TrayHideWindowTooltip = "Hide window"

	TrayQuitTitle   = "Quit"
	TrayQuitTooltip = "Exit the application"
)

// Search Editor Strings
const SearchEditorHint = "Search commands..."

// Hint widget Strings — base hints without hotkey suffix.
var baseHints = func() [4]string {
	mod := hotkey.Ctrl
	if platform.IsMacOS() {
		mod = hotkey.Cmd
	}

	return [4]string{
		model.TabCommands:    "↑↓ to navigate · " + mod + "+↓ to last · Enter to copy",
		model.TabTree:        "↑↓ to navigate · " + mod + "+↑↓ prev/next branch · Enter to copy",
		model.TabTopCommands: "↑↓ to navigate · Enter to copy",
		model.TabSettings:    "",
	}
}()

// hints holds pre-computed hint strings including the hotkey suffix.
// Initialized from baseHints; rebuilt by RebuildHints when the active preset changes.
var hints = baseHints

// RebuildHints pre-computes the per-tab hint strings with the given hotkey
// display name appended. Call once at startup and whenever the preset changes.
func RebuildHints(hotkeyDisplayName string) {
	suffix := ""
	if hotkeyDisplayName != "" {
		suffix = " · " + hotkeyDisplayName + " to toggle"
	}

	for i, base := range baseHints {
		if base != "" {
			hints[i] = base + suffix
		}
	}
}

func tabHint(t model.Tab) string {
	return hints[t]
}

// Tab names
func tabName(t model.Tab) string {
	switch t {
	case model.TabCommands:
		return "Commands"
	case model.TabTree:
		return "Tree"
	case model.TabTopCommands:
		return "Top Commands"
	case model.TabSettings:
		return "Settings"
	default:
		return "Unknown"
	}
}

// Tabs shortcuts
var tabShortcutHints = func() [4]string {
	prefix := hotkey.Ctrl
	if platform.IsMacOS() {
		prefix = hotkey.Cmd
	}

	return [4]string{
		model.TabCommands:    prefix + "+1",
		model.TabTree:        prefix + "+2",
		model.TabTopCommands: prefix + "+3",
		model.TabSettings:    " ",
	}
}()

// tabShortcutHint returns the keyboard shortcut hint for the given tab type.
func tabShortcutHint(t model.Tab) string {
	return tabShortcutHints[t]
}

// Top Commands Tab Strings
const (
	TopCommandsLoading     = "Calculating statistics..."
	TopCommandsTop10Title  = "Top 10 Commands"
	TopCommandsTopPrefixes = "Top Prefixes"
)

// Settings Tab
const (
	HotKeyCardTitle           = "Global Hotkey"
	HotKeyCardDescription     = "Configure the hotkey to show/hide the application"
	HotKeyCardAction          = "Choose Hotkey Preset:"
	HotKeyCardSuccess         = "Hotkey updated to "
	HotKeyCardFailure         = "Failed to register hotkey: %v"
	HotKeyCardFailureDueToCfg = "Warning: Hotkey updated but config save failed: %v"

	AutoStartCardTitle       = "Start on Boot"
	AutoStartCardDescription = "Automatically start Debrief when you log in to your computer"
	AutoStartSuccess         = "Autostart updated successfully"
	AutoStartFailure         = "Failed to update autostart: %v"
	AutoStartFailureCfg      = "Warning: Autostart updated but config save failed: %v"
)

// ---
// Top Commands Tab
const (
	// StatsFadeWidth is the width of the fade-out zone for long commands in statistics
	StatsFadeWidth = unit.Dp(40)
	// StatsFadeSteps is the number of gradient strips for the statistics fade effect
	StatsFadeSteps = 8
)

//----
// Shell badges

// MaxShellBadges is the maximum number of shell filter badges rendered.
const MaxShellBadges = 10

// ---
// WINDOW
// (in Dp units)
const (
	MinWidth  = unit.Dp(500)
	MinHeight = unit.Dp(400)

	// Height of the draggable area in dp
	dragAreaHeight = 40
)

// ---
// SPACING
// (in Dp units)
const (
	// SpacingTiny is the smallest spacing unit
	SpacingTiny = unit.Dp(2)
	// SpacingXSmall is an extra small spacing unit
	SpacingXSmall = unit.Dp(4)
	// SpacingSmall is a small spacing unit
	SpacingSmall = unit.Dp(6)
	// SpacingMedium is a medium spacing unit
	SpacingMedium = unit.Dp(8)
	// SpacingLarge is a large spacing unit
	SpacingLarge = unit.Dp(12)
	// SpacingXLarge is an extra large spacing unit
	SpacingXLarge = unit.Dp(16)
	// SpacingXXLarge is a double extra large spacing unit
	SpacingXXLarge = unit.Dp(20)
	// SpacingHuge is a huge spacing unit
	SpacingHuge = unit.Dp(24)
	// SpacingMassive is a massive spacing unit
	SpacingMassive = unit.Dp(32)
	// SpacingLargeInset is used for large insets
	SpacingLargeInset = unit.Dp(40)
	// SpacingXLargeInset is used for extra large insets
	SpacingXLargeInset = unit.Dp(60)
)

// ---
// COLOR
// (RGB values)
const (
	// ColorDarkGray40 is the RGB value for darker gray backgrounds (tabs, cards)
	ColorDarkGray40 = 40
	// ColorDarkGray50 is the RGB value for medium dark gray
	ColorDarkGray50 = 50
	// ColorDarkGray60 is the RGB value for dark gray background
	ColorDarkGray60 = 60
	// ColorBlueRed is the RGB red channel value for blue background
	ColorBlueRed = 100
	// ColorBlueGreen is the RGB green channel value for blue background
	ColorBlueGreen = 150
	// ColorGray180 is the RGB value for lighter gray text
	ColorGray180 = 180
	// ColorGray200 is the RGB value for light gray text
	ColorGray200 = 200
	// ColorGray220 is the RGB value for lighter gray text
	ColorGray220 = 220
	// ColorBlueBlue is the RGB blue channel value for blue background
	ColorBlueBlue = 255
	// ColorBlueAlpha is the alpha channel value for blue background
	ColorBlueAlpha = 200
	// ColorGrayAlpha is the alpha channel value for gray background
	ColorGrayAlpha = 100
	// ColorGrayAlpha40 is a low opacity alpha for hover states
	ColorGrayAlpha40 = 40
	// ColorAlpha60 is a medium opacity alpha for backgrounds
	ColorAlpha60 = 60
	// ColorAlpha230 is a high opacity alpha for selected items
	ColorAlpha230 = 230
	// ColorWhite is the RGB value for white
	ColorWhite = 255
	// ColorStatusGreen is the RGB green channel value for enabled status
	ColorStatusGreen = 150
	// ColorStatusRed is the RGB red channel value for disabled status
	ColorStatusRed = 150
	// ColorErrorRed is the RGB red channel value for error messages
	ColorErrorRed = 255
	// ColorErrorGreen is the RGB green channel value for error messages
	ColorErrorGreen = 100
	// ColorErrorBlue is the RGB blue channel value for error messages
	ColorErrorBlue = 100
	// ColorSuccessRed is the RGB red channel value for success messages
	ColorSuccessRed = 100
	// ColorSuccessGreen is the RGB green channel value for success messages
	ColorSuccessGreen = 255
	// ColorSuccessBlue is the RGB blue channel value for success messages
	ColorSuccessBlue = 100
)

// ---
// SIZE
const (
	// BorderRadius is the radius for rounded corners
	BorderRadius = unit.Dp(4)
	// TreeRowHeight is the minimum height of tree view rows
	TreeRowHeight = unit.Dp(40)
	// TreeRowInsetHeight is the additional height for tree row insets
	TreeRowInsetHeight = unit.Dp(2)
	// TreeIconWidth is the width for tree node icons
	TreeIconWidth = unit.Dp(20)
	// TreeIconSize is the size for tree node vector icons
	TreeIconSize = unit.Dp(18)
	// TreeContentTopPadding is the top padding to vertically center content in rows
	TreeContentTopPadding = unit.Dp(11)
	// TreeIndentBase is the base left indent for tree nodes
	TreeIndentBase = 8
	// TreeIndentMultiplier is the indent multiplier per tree depth level
	TreeIndentMultiplier = 16
	// TreeOpacityBase is the base opacity for tree view ancestors
	TreeOpacityBase = 150
	// TreeOpacityRange is the opacity range for tree view ancestors
	TreeOpacityRange = 105
	// TreePrefixMaxWidthPercent is the max percentage of row width the prefix can occupy (out of 100)
	TreePrefixMaxWidthPercent = 60
	// TreePrefixPercentBase is the denominator for percentage calculations
	TreePrefixPercentBase = 100
	// TreePrefixFadeWidth is the width of the fade-out zone at the end of a truncated prefix
	TreePrefixFadeWidth = unit.Dp(40)
	// TreePrefixFadeSteps is the number of gradient strips for the fade effect
	TreePrefixFadeSteps = 8
	// CircleIndicatorSize is the size for circular radio button indicators
	CircleIndicatorSize = unit.Dp(16)
	// CheckboxSize is the size for checkbox indicators
	CheckboxSize = unit.Dp(18)
	// CheckboxRadius is the corner radius for checkbox indicators
	CheckboxRadius = unit.Dp(3)
	// CheckboxMarginDivisor controls X mark inset relative to checkbox size (size / 4 = 25% margin)
	CheckboxMarginDivisor = 4
	// CheckboxStrokeDivisor controls X mark stroke width relative to checkbox size (size / 8)
	CheckboxStrokeDivisor = 8
	// searchEditorMaxHeight is the max height for the search editor in dp
	searchEditorMaxHeight = 28
	// itemsPerPage is the number of items to jump on PageUp/PageDown in command/stat lists
	itemsPerPage = 10
	// nodesPerPage is the number of nodes to jump on PageUp/PageDown in tree view
	nodesPerPage = 10
)
