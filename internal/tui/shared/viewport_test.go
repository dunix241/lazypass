package shared

import "testing"

func TestListViewportKeepsQuarterMarginWhenReversing(t *testing.T) {
	var viewport ListViewport
	for selected := 0; selected <= 20; selected++ {
		viewport.Window(40, 8, selected)
	}
	start, end := viewport.Window(40, 8, 20)
	if start != 15 || end != 23 {
		t.Fatalf("window = %d:%d", start, end)
	}
	for selected := 19; selected >= 17; selected-- {
		start, _ = viewport.Window(40, 8, selected)
		if start != 15 {
			t.Fatal("viewport moved before selection reached upper margin")
		}
	}
	start, _ = viewport.Window(40, 8, 16)
	if start != 14 {
		t.Fatal("viewport did not scroll at upper margin")
	}
	if again, _ := viewport.Window(40, 8, 16); again != start {
		t.Fatal("repeated layout changed window")
	}
}

func TestListViewportBoundsResizeAndShrink(t *testing.T) {
	var viewport ListViewport
	viewport.Window(100, 10, 99)
	for _, tc := range []struct{ count, height, selected int }{
		{3, 10, 99}, {100, 1, 50}, {100, 20, 50}, {0, 10, 0}, {10, 0, 5},
	} {
		start, end := viewport.Window(tc.count, tc.height, tc.selected)
		if start < 0 || end < start || end > tc.count || end-start > max(0, tc.height) {
			t.Fatalf("invalid window %d:%d", start, end)
		}
	}
}

func TestListViewportSelectionVisibleAcrossFullNavigationAndResize(t *testing.T) {
	for count := 1; count <= 64; count++ {
		var viewport ListViewport
		for _, height := range []int{1, 2, 3, 4, 8, 13, 24, 5} {
			for _, direction := range []int{1, -1} {
				for offset := 0; offset < count; offset++ {
					selected := offset
					if direction < 0 {
						selected = count - offset - 1
					}
					start, end := viewport.Window(count, height, selected)
					if selected < start || selected >= end || end-start != min(count, height) {
						t.Fatalf("count=%d height=%d selected=%d window=%d:%d", count, height, selected, start, end)
					}
				}
			}
		}
	}
}
