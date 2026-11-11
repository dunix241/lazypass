package shared

// ListViewport retains the scroll position and keeps selection within a quarter-
// viewport margin. Drawing and hit testing must use the same instance.
type ListViewport struct {
	start int
}

func (v *ListViewport) Window(count, height, selected int) (start, end int) {
	if count <= 0 || height <= 0 {
		v.start = 0
		return 0, 0
	}
	height = min(height, count)
	selected = max(0, min(selected, count-1))
	margin := min(height/4, (height-1)/2)
	v.start = max(0, min(v.start, count-height))
	if selected < v.start+margin {
		v.start = max(0, selected-margin)
	} else if selected >= v.start+height-margin {
		v.start = min(count-height, selected-height+margin+1)
	}
	return v.start, min(count, v.start+height)
}
