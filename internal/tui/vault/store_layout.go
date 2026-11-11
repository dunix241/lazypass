package vault

import "lazypass/internal/tui/shared"

type storeFieldRow struct {
	index  int
	y      int
	height int
}

func (s *StoreForm) fieldWidth(panel shared.Rect, field entryField) int {
	width := panel.W - 6 - len(field.label)
	return max(1, width)
}

func (s *StoreForm) fieldRows(panel shared.Rect) []storeFieldRow {
	fields := s.entryFields()
	capacity := max(1, panel.H-6)
	heights := make([]int, len(fields))
	total, selectedEnd := 0, 0
	for i, field := range fields {
		heights[i] = 1
		if field.multiline {
			heights[i] = min(capacity, s.editor(field.key, s.displayField(i)).Rows(s.fieldWidth(panel, field), field.inputOptions()))
		}
		total += heights[i]
		if i == s.field {
			selectedEnd = total - 1
		}
	}
	startRow, _ := s.fieldViewport.Window(total, capacity, selectedEnd)
	start, offset := 0, 0
	for start < s.field && offset+heights[start] <= startRow {
		offset += heights[start]
		start++
	}
	for start < s.field && selectedEnd-offset >= capacity {
		offset += heights[start]
		start++
	}
	rows := make([]storeFieldRow, 0, len(fields)-start)
	y, remaining := panel.Y+4, capacity
	for i := start; i < len(fields) && remaining > 0; i++ {
		height := min(heights[i], remaining)
		rows = append(rows, storeFieldRow{index: i, y: y, height: height})
		y += height
		remaining -= height
	}
	return rows
}
