package ui

const safetyMarginPx = 20

// estimateFallbackHeight computes a fallback height for unmeasured items.
// It averages all non-zero measured heights in the cache. If none have been
// measured yet, it falls back to the minimum row height constant.
// This prevents multi-line items from causing wildly wrong scroll positions
// when the fixed minimum (≈42dp) is used for items that are actually 2-3x taller.
func estimateFallbackHeight(gtx C, itemHeights []int) int {
	sum, count := 0, 0

	for _, h := range itemHeights {
		if h > 0 {
			sum += h
			count++
		}
	}

	if count > 0 {
		return sum / count
	}

	// Nothing measured yet — use the minimum row constant
	return gtx.Dp(TreeRowHeight) + gtx.Dp(TreeRowInsetHeight)
}

// calculateSmartScrollPositionVariable determines scroll position for variable-height items using cached heights.
// It sums actual rendered heights to determine visible range, supporting multi-line content.
// Returns (newFirstPosition, shouldScroll).
func calculateSmartScrollPositionVariable(gtx C, currentFirst, selectedIndex int, itemHeights []int) (int, bool) {
	// Validate inputs
	if selectedIndex < 0 || len(itemHeights) == 0 || selectedIndex >= len(itemHeights) {
		return currentFirst, false
	}

	if currentFirst < 0 {
		currentFirst = 0
	}

	if currentFirst >= len(itemHeights) {
		currentFirst = len(itemHeights) - 1
	}

	viewportHeight := gtx.Constraints.Max.Y
	if viewportHeight <= 0 {
		return currentFirst, false
	}

	fallbackHeight := estimateFallbackHeight(gtx, itemHeights)

	// Calculate which items are currently visible
	currentVisibleHeight := 0
	lastFittingIndex := currentFirst

	for i := currentFirst; i < len(itemHeights); i++ {
		height := itemHeights[i]
		if height <= 0 {
			height = fallbackHeight
		}

		if currentVisibleHeight+height > viewportHeight-safetyMarginPx {
			break
		}

		currentVisibleHeight += height
		lastFittingIndex = i
	}

	// Check if selected item is already visible
	if selectedIndex >= currentFirst && selectedIndex <= lastFittingIndex {
		return currentFirst, false // No scroll needed
	}

	// Selected item is above viewport - scroll up to show it at top
	if selectedIndex < currentFirst {
		return selectedIndex, true
	}

	// Selected item is below viewport - scroll down to show it near bottom
	// Calculate new first index by working backward from selected item
	newFirst := selectedIndex
	accumulatedHeight := 0

	// Guard: if the selected item alone exceeds the viewport, show it at the top.
	selectedItemHeight := itemHeights[selectedIndex]
	if selectedItemHeight <= 0 {
		selectedItemHeight = fallbackHeight
	}

	if selectedItemHeight >= viewportHeight-safetyMarginPx {
		return selectedIndex, true
	}

	// Start from selected item and work backward until we fill the viewport
	for i := selectedIndex; i >= 0; i-- {
		height := itemHeights[i]
		if height <= 0 {
			height = fallbackHeight
		}

		if accumulatedHeight+height > viewportHeight-safetyMarginPx {
			// Found the first item that fits
			newFirst = i + 1
			break
		}

		accumulatedHeight += height
		newFirst = i
	}

	// Ensure newFirst is valid
	if newFirst < 0 {
		newFirst = 0
	}

	if newFirst >= len(itemHeights) {
		newFirst = len(itemHeights) - 1
	}

	return newFirst, true
}
