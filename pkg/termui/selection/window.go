package selection

// Window returns a half-open row range [start, end) for a viewport. Offset is
// clamped to keep a full viewport when possible. Negative sizes are treated as
// zero; a zero-height viewport has start == end. The result can slice total rows.
func Window(offset, visible, total int) (start, end int) {
	total = max(0, total)
	visible = max(0, visible)
	start = min(max(0, offset), max(0, total-visible))

	return start, start + min(visible, total-start)
}
