package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// editorState is the shared VS Code-like editing state for UTILS tabs.
// Prefer this (not ad-hoc key handlers) for any new utility editors.
type editorState struct {
	Cursor  int
	Anchor  int // -1 = no selection
	VScroll int
	HScroll int
}

func (e *editorState) clearSel() {
	e.Anchor = -1
}

func (e *editorState) selRange(editing bool) (lo, hi int, ok bool) {
	if !editing || e.Anchor < 0 {
		return 0, 0, false
	}
	lo, hi = e.Anchor, e.Cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == hi {
		return 0, 0, false
	}
	return lo, hi, true
}

func (e *editorState) deleteSelection(text string, editing bool) (string, bool) {
	lo, hi, ok := e.selRange(editing)
	if !ok {
		return text, false
	}
	runes := []rune(text)
	if lo > len(runes) {
		lo = len(runes)
	}
	if hi > len(runes) {
		hi = len(runes)
	}
	text = string(append(runes[:lo], runes[hi:]...))
	e.Cursor = lo
	e.clearSel()
	return text, true
}

func (e *editorState) move(textLen, next int, extend bool) {
	if next < 0 {
		next = 0
	}
	if next > textLen {
		next = textLen
	}
	if extend {
		if e.Anchor < 0 {
			e.Anchor = e.Cursor
		}
	} else {
		e.clearSel()
	}
	e.Cursor = next
}

// editorApplyKey handles VS Code-like navigation/edit. Returns new text and whether the key was handled.
func editorApplyKey(msg tea.KeyMsg, text string, e *editorState, multiline bool) (string, bool) {
	runes := []rune(text)
	if e.Cursor < 0 {
		e.Cursor = 0
	}
	if e.Cursor > len(runes) {
		e.Cursor = len(runes)
	}
	key := msg.String()
	cursor := e.Cursor

	switch key {
	case "ctrl+a":
		if len(runes) == 0 {
			e.clearSel()
			return text, true
		}
		e.Anchor = 0
		e.Cursor = len(runes)
		return text, true
	case "ctrl+c":
		if lo, hi, ok := e.selRange(true); ok {
			_ = copyToClipboard(string(runes[lo:hi]))
		}
		return text, true
	case "ctrl+x":
		if lo, hi, ok := e.selRange(true); ok {
			_ = copyToClipboard(string(runes[lo:hi]))
			text, _ = e.deleteSelection(text, true)
		}
		return text, true
	case "ctrl+v":
		clip, err := readClipboard()
		if err != nil {
			return text, true
		}
		if t, ok := e.deleteSelection(text, true); ok {
			text = t
			runes = []rune(text)
			cursor = e.Cursor
		}
		ins := []rune(clip)
		runes = append(runes[:cursor], append(ins, runes[cursor:]...)...)
		e.Cursor = cursor + len(ins)
		e.clearSel()
		return string(runes), true
	case "enter":
		if !multiline {
			return text, false
		}
		if t, ok := e.deleteSelection(text, true); ok {
			text = t
			runes = []rune(text)
			cursor = e.Cursor
		}
		indent := apiLineIndent(runes, cursor)
		prev := apiRuneBefore(runes, cursor)
		next := apiRuneAfter(runes, cursor)
		if (prev == '{' && next == '}') || (prev == '[' && next == ']') {
			insert := []rune("\n" + indent + "  \n" + indent)
			runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
			e.Cursor = cursor + len([]rune("\n"+indent+"  "))
			e.clearSel()
			return string(runes), true
		}
		if prev == '{' || prev == '[' {
			indent += "  "
		}
		insert := []rune("\n" + indent)
		runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
		e.Cursor = cursor + len(insert)
		e.clearSel()
		return string(runes), true
	case "left":
		if lo, _, ok := e.selRange(true); ok {
			e.move(len(runes), lo, false)
		} else {
			e.move(len(runes), cursor-1, false)
		}
		return text, true
	case "right":
		if _, hi, ok := e.selRange(true); ok {
			e.move(len(runes), hi, false)
		} else {
			e.move(len(runes), cursor+1, false)
		}
		return text, true
	case "shift+left":
		e.move(len(runes), cursor-1, true)
		return text, true
	case "shift+right":
		e.move(len(runes), cursor+1, true)
		return text, true
	case "ctrl+left":
		e.move(len(runes), apiMoveWordLeft(runes, cursor), false)
		return text, true
	case "ctrl+right":
		e.move(len(runes), apiMoveWordRight(runes, cursor), false)
		return text, true
	case "ctrl+shift+left":
		e.move(len(runes), apiMoveWordLeft(runes, cursor), true)
		return text, true
	case "ctrl+shift+right":
		e.move(len(runes), apiMoveWordRight(runes, cursor), true)
		return text, true
	case "up":
		if multiline {
			e.move(len(runes), apiMoveLine(runes, cursor, -1), false)
			return text, true
		}
		return text, false
	case "down":
		if multiline {
			e.move(len(runes), apiMoveLine(runes, cursor, 1), false)
			return text, true
		}
		return text, false
	case "shift+up":
		if multiline {
			e.move(len(runes), apiMoveLine(runes, cursor, -1), true)
			return text, true
		}
		return text, false
	case "shift+down":
		if multiline {
			e.move(len(runes), apiMoveLine(runes, cursor, 1), true)
			return text, true
		}
		return text, false
	case "home":
		e.move(len(runes), apiLineStart(runes, cursor), false)
		return text, true
	case "end":
		e.move(len(runes), apiLineEnd(runes, cursor), false)
		return text, true
	case "shift+home":
		e.move(len(runes), apiLineStart(runes, cursor), true)
		return text, true
	case "shift+end":
		e.move(len(runes), apiLineEnd(runes, cursor), true)
		return text, true
	case "ctrl+home":
		e.move(len(runes), 0, false)
		return text, true
	case "ctrl+end":
		e.move(len(runes), len(runes), false)
		return text, true
	case "ctrl+shift+home":
		e.move(len(runes), 0, true)
		return text, true
	case "ctrl+shift+end":
		e.move(len(runes), len(runes), true)
		return text, true
	case "backspace":
		if t, ok := e.deleteSelection(text, true); ok {
			return t, true
		}
		if cursor > 0 {
			runes = append(runes[:cursor-1], runes[cursor:]...)
			e.Cursor = cursor - 1
			return string(runes), true
		}
		return text, true
	case "delete":
		if t, ok := e.deleteSelection(text, true); ok {
			return t, true
		}
		if cursor < len(runes) {
			runes = append(runes[:cursor], runes[cursor+1:]...)
			return string(runes), true
		}
		return text, true
	case "tab":
		if multiline {
			if t, ok := e.deleteSelection(text, true); ok {
				text = t
				runes = []rune(text)
				cursor = e.Cursor
			}
			insert := []rune("  ")
			runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
			e.Cursor = cursor + len(insert)
			e.clearSel()
			return string(runes), true
		}
		return text, false
	case "shift+tab":
		if multiline {
			e.clearSel()
			n := apiUnindentAt(runes, cursor)
			if n > 0 {
				runes = append(runes[:cursor-n], runes[cursor:]...)
				e.Cursor = cursor - n
				return string(runes), true
			}
			return text, true
		}
		return text, false
	default:
		var inserted []rune
		if len(msg.Runes) > 0 {
			inserted = msg.Runes
		} else if s := key; len(s) == 1 {
			inserted = []rune(s)
		}
		if len(inserted) == 0 {
			return text, false
		}
		if t, ok := e.deleteSelection(text, true); ok {
			text = t
			runes = []rune(text)
			cursor = e.Cursor
		}
		runes = append(runes[:cursor], append(inserted, runes[cursor:]...)...)
		e.Cursor = cursor + len(inserted)
		e.clearSel()
		return string(runes), true
	}
}

// renderEditorLines renders text with cursor, selection, optional JSON highlight and HScroll.
func renderEditorLines(text string, e *editorState, width, height int, editing, highlightJSON bool) []string {
	runes := []rune(text)
	cursor := e.Cursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	selLo, selHi, hasSel := e.selRange(editing)
	var kinds []uint8
	if highlightJSON {
		kinds = jsonKindsForRunes(text)
	}

	type lineSpan struct{ start, end int }
	var spans []lineSpan
	start := 0
	for i, r := range runes {
		if r == '\n' {
			spans = append(spans, lineSpan{start, i})
			start = i + 1
		}
	}
	spans = append(spans, lineSpan{start, len(runes)})

	cursorLine := 0
	cursorCol := 0
	for i, sp := range spans {
		if cursor <= sp.end {
			cursorLine = i
			cursorCol = cursor - sp.start
			break
		}
		if i == len(spans)-1 {
			cursorLine = i
			cursorCol = cursor - sp.start
		}
	}

	// Keep cursor visible horizontally.
	if editing {
		if cursorCol < e.HScroll {
			e.HScroll = cursorCol
		}
		if cursorCol >= e.HScroll+width {
			e.HScroll = cursorCol - width + 1
		}
		if e.HScroll < 0 {
			e.HScroll = 0
		}
	}

	e.VScroll = ensureVisible(cursorLine, e.VScroll, height, len(spans))
	from := e.VScroll
	to := minInt(from+height, len(spans))

	out := make([]string, 0, height)
	for _, sp := range spans[from:to] {
		lineRunes := runes[sp.start:sp.end]
		h := e.HScroll
		if h > len(lineRunes) {
			h = len(lineRunes)
		}
		var b strings.Builder
		lineCursor := cursor - sp.start
		showCursorHere := editing && cursor >= sp.start && cursor <= sp.end

		if showCursorHere && lineCursor == 0 && len(lineRunes) == 0 && h == 0 {
			b.WriteRune('█')
		}
		end := minInt(h+width, len(lineRunes))
		for i := h; i < end; i++ {
			abs := sp.start + i
			if showCursorHere && i == lineCursor {
				b.WriteRune('█')
			}
			s := string(lineRunes[i])
			switch {
			case hasSel && abs >= selLo && abs < selHi:
				s = StyleApiSel.Render(s)
			case highlightJSON && abs < len(kinds):
				s = styleJSONRune(kinds[abs], s)
			}
			b.WriteString(s)
		}
		if showCursorHere && lineCursor >= h && lineCursor == len(lineRunes) {
			if len(lineRunes) > 0 || h > 0 {
				b.WriteRune('█')
			}
		}
		out = append(out, ansi.Truncate(b.String(), width, "…"))
	}
	return fitExactLines(out, height)
}

func paneTitleWithHScroll(base string, hScroll int) string {
	if hScroll <= 0 {
		return "[" + base + "]"
	}
	return "[" + base + " · ←" + strconv.Itoa(hScroll) + "]"
}

func hScrollDelta(cur, delta, maxLineLen, width int) int {
	next := cur + delta
	if next < 0 {
		return 0
	}
	maxH := maxInt(0, maxLineLen-width)
	if next > maxH {
		return maxH
	}
	return next
}

func maxLineRuneLen(text string) int {
	max := 0
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if n := len([]rune(line)); n > max {
			max = n
		}
	}
	return max
}
