package ui

import "testing"

func TestPageJump(t *testing.T) {
	tests := []struct {
		name                     string
		current, total, pageSize int
		up                       bool
		want                     int
	}{
		// No selection
		{"no selection page up", -1, 20, 10, true, 19},
		{"no selection page down", -1, 20, 10, false, 0},
		// Normal movement
		{"page down from middle", 5, 20, 10, false, 15},
		{"page up from middle", 15, 20, 10, true, 5},
		// Clamp at boundaries
		{"page up clamps at 0", 3, 20, 10, true, 0},
		{"page down clamps at last", 18, 20, 10, false, 19},
		// Empty list
		{"empty list page up", -1, 0, 10, true, -1},
		{"empty list page down", 0, 0, 10, false, 0},
		// Single item
		{"single item page up", -1, 1, 10, true, 0},
		{"single item page down", 0, 1, 10, false, 0},
		// Already at boundary
		{"at first page up", 0, 10, 5, true, 0},
		{"at last page down", 9, 10, 5, false, 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pageJump(tt.current, tt.total, tt.pageSize, tt.up)
			if got != tt.want {
				t.Errorf("pageJump(%d, %d, %d, %v) = %d, want %d",
					tt.current, tt.total, tt.pageSize, tt.up, got, tt.want)
			}
		})
	}
}
