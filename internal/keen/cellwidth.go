package keen

import "strings"

// runeCellWidth returns the terminal display cell width of r (0 for zero-width/combining,
// 1 for normal ASCII/Latin, 2 for CJK and wide emoji/symbols). It provides dependency-free
// terminal-cell-aware layout without pulling in external packages.
func runeCellWidth(r rune) int {
	if r == 0 {
		return 0
	}
	// Zero width characters, combining marks, control characters
	if r < 32 || (r >= 0x7f && r < 0xa0) || (r >= 0x300 && r <= 0x36f) || (r >= 0x200b && r <= 0x200f) || (r >= 0x2028 && r <= 0x202e) {
		return 0
	}
	// Common CJK ranges, Hangul, Fullwidth forms, and common emoji/symbols
	if (r >= 0x1100 && r <= 0x115f) ||
		(r >= 0x231a && r <= 0x231b) ||
		(r >= 0x2e80 && r <= 0x303e) ||
		(r >= 0x3040 && r <= 0x33ff) ||
		(r >= 0x3400 && r <= 0x4dbf) ||
		(r >= 0x4e00 && r <= 0x9fff) ||
		(r >= 0xa000 && r-0xa000 <= 0x48ff) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x20000 && r <= 0x2a6df) ||
		(r >= 0x2a700 && r <= 0x2b73f) ||
		(r >= 0x2b740 && r <= 0x2b81f) ||
		(r >= 0x2b820 && r <= 0x2ceaf) ||
		(r >= 0x2f800 && r <= 0x2fa1f) ||
		(r >= 0x1f000 && r <= 0x1faff) {
		return 2
	}
	return 1
}

// stringCellWidth returns the total terminal display cell width of s.
func stringCellWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeCellWidth(r)
	}
	return w
}

// truncateCells shortens s to at most maxCells terminal display cells, appending
// an ellipsis when the content is longer. It is cell-width and rune-aware so
// CJK/emoji and multi-byte characters are never split or misaligned.
func truncateCells(s string, maxCells int) string {
	if stringCellWidth(s) <= maxCells {
		return s
	}
	if maxCells <= 1 {
		if maxCells == 1 {
			return "…"
		}
		return ""
	}
	var sb strings.Builder
	cur := 0
	target := maxCells - 1 // reserve 1 cell for '…'
	for _, r := range s {
		rw := runeCellWidth(r)
		if cur+rw > target {
			break
		}
		sb.WriteRune(r)
		cur += rw
	}
	sb.WriteRune('…')
	return sb.String()
}

// padCells pads s with spaces to exactly width terminal display cells, aligning
// left or right depending on right flag.
func padCells(s string, width int, right bool) string {
	cw := stringCellWidth(s)
	if cw >= width {
		return truncateCells(s, width)
	}
	pad := strings.Repeat(" ", width-cw)
	if right {
		return pad + s
	}
	return s + pad
}
