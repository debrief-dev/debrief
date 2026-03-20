package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// makeGtx creates a minimal layout.Context with the given viewport height.
// Uses 1:1 dp-to-px ratio for test simplicity.
func makeGtx(viewportHeight int) C {
	var ops op.Ops

	return layout.Context{
		Ops: &ops,
		Constraints: layout.Constraints{
			Max: image.Pt(800, viewportHeight),
		},
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
}

func TestCalculateSmartScrollPositionVariable_AlreadyVisible(t *testing.T) {
	gtx := makeGtx(300)
	heights := []int{50, 50, 50, 50, 50, 50}

	// Item 2 is visible when first=0 (items 0-4 fit in 300px with 20px margin)
	newFirst, shouldScroll := calculateSmartScrollPositionVariable(gtx, 0, 2, heights)
	if shouldScroll {
		t.Errorf("Expected no scroll needed, got newFirst=%d", newFirst)
	}
}

func TestCalculateSmartScrollPositionVariable_AboveViewport(t *testing.T) {
	gtx := makeGtx(300)
	heights := []int{50, 50, 50, 50, 50, 50, 50, 50, 50, 50}

	// currentFirst=5, selected=2 (above viewport)
	newFirst, shouldScroll := calculateSmartScrollPositionVariable(gtx, 5, 2, heights)
	if !shouldScroll {
		t.Fatal("Expected scroll needed")
	}

	if newFirst != 2 {
		t.Errorf("Expected newFirst=2, got %d", newFirst)
	}
}

func TestCalculateSmartScrollPositionVariable_BelowViewport(t *testing.T) {
	gtx := makeGtx(200)
	heights := []int{50, 50, 50, 50, 50, 50, 50, 50, 50, 50}

	// currentFirst=0, selected=8 (below viewport - only ~3 items visible in 200-20=180px)
	newFirst, shouldScroll := calculateSmartScrollPositionVariable(gtx, 0, 8, heights)
	if !shouldScroll {
		t.Fatal("Expected scroll needed")
	}
	// Should scroll so item 8 is near the bottom
	if newFirst > 8 || newFirst < 5 {
		t.Errorf("Expected newFirst between 5 and 8, got %d", newFirst)
	}
}

func TestCalculateSmartScrollPositionVariable_EmptyHeights(t *testing.T) {
	gtx := makeGtx(300)

	newFirst, shouldScroll := calculateSmartScrollPositionVariable(gtx, 0, 0, []int{})
	if shouldScroll {
		t.Errorf("Expected no scroll for empty heights, got newFirst=%d", newFirst)
	}
}

func TestCalculateSmartScrollPositionVariable_NegativeSelected(t *testing.T) {
	gtx := makeGtx(300)
	heights := []int{50, 50, 50}

	newFirst, shouldScroll := calculateSmartScrollPositionVariable(gtx, 0, -1, heights)
	if shouldScroll {
		t.Errorf("Expected no scroll for negative index, got newFirst=%d", newFirst)
	}
}

func TestCalculateSmartScrollPositionVariable_LargeItemExceedsViewport(t *testing.T) {
	gtx := makeGtx(100)
	heights := []int{50, 50, 200, 50} // item 2 is larger than viewport

	newFirst, shouldScroll := calculateSmartScrollPositionVariable(gtx, 0, 2, heights)
	if !shouldScroll {
		t.Fatal("Expected scroll needed for large item")
	}
	// When item exceeds viewport, it should be placed at the top
	if newFirst != 2 {
		t.Errorf("Expected newFirst=2 for large item, got %d", newFirst)
	}
}

func TestCalculateSmartScrollPositionVariable_ZeroViewport(t *testing.T) {
	gtx := makeGtx(0)
	heights := []int{50, 50, 50}

	_, shouldScroll := calculateSmartScrollPositionVariable(gtx, 0, 1, heights)
	if shouldScroll {
		t.Error("Expected no scroll for zero viewport")
	}
}

func TestCalculateSmartScrollPositionVariable_UnmeasuredItems(t *testing.T) {
	gtx := makeGtx(300)
	// Some items unmeasured (0 height) — should use fallback
	heights := []int{50, 0, 0, 50, 0, 50, 50, 50, 50, 50}

	// Should not panic and should produce valid result
	newFirst, _ := calculateSmartScrollPositionVariable(gtx, 0, 8, heights)
	if newFirst < 0 || newFirst > 8 {
		t.Errorf("Expected valid newFirst, got %d", newFirst)
	}
}

func TestEstimateFallbackHeight_AllMeasured(t *testing.T) {
	gtx := makeGtx(300)
	heights := []int{40, 60, 50}
	got := estimateFallbackHeight(gtx, heights)

	want := 50 // (40+60+50)/3
	if got != want {
		t.Errorf("estimateFallbackHeight = %d, want %d", got, want)
	}
}

func TestEstimateFallbackHeight_NoneMeasured(t *testing.T) {
	gtx := makeGtx(300)
	heights := []int{0, 0, 0}
	got := estimateFallbackHeight(gtx, heights)
	// Should use TreeRowHeight + TreeRowInsetHeight via gtx.Dp
	if got <= 0 {
		t.Errorf("estimateFallbackHeight = %d, want > 0", got)
	}
}
