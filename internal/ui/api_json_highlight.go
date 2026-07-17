package ui

import (
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// ponytail: tiny JSON lexer for Body colors — not a full parser; broken JSON still paints best-effort.

const (
	jsonKindPlain uint8 = iota
	jsonKindKey
	jsonKindString
	jsonKindNumber
	jsonKindKeyword // true/false/null
	jsonKindPunct   // {}[]
	jsonKindSep     // :,
)

func jsonKindStyle(kind uint8) lipgloss.Style {
	switch kind {
	case jsonKindKey:
		return StyleJSONKey
	case jsonKindString:
		return StyleJSONString
	case jsonKindNumber:
		return StyleJSONNumber
	case jsonKindKeyword:
		return StyleJSONKeyword
	case jsonKindPunct:
		return StyleJSONPunct
	case jsonKindSep:
		return StyleJSONSep
	default:
		return StyleNormal
	}
}

func styleJSONRune(kind uint8, s string) string {
	if s == "" {
		return s
	}
	return jsonKindStyle(kind).Render(s)
}

// jsonTokenKinds returns one kind per byte index (UTF-8 safe: non-ASCII continuation bytes get plain).
func jsonTokenKinds(s string) []uint8 {
	n := len(s)
	kinds := make([]uint8, n)
	i := 0
	for i < n {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			kinds[i] = jsonKindPlain
			i++
		case '{', '}', '[', ']':
			kinds[i] = jsonKindPunct
			i++
		case ':', ',':
			kinds[i] = jsonKindSep
			i++
		case '"':
			start := i
			i++
			for i < n {
				if s[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if s[i] == '"' {
					i++
					break
				}
				i++
			}
			kind := jsonKindString
			j := i
			for j < n && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j < n && s[j] == ':' {
				kind = jsonKindKey
			}
			for k := start; k < i; k++ {
				kinds[k] = kind
			}
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			start := i
			if s[i] == '-' {
				i++
			}
			for i < n {
				c := s[i]
				if c >= '0' && c <= '9' || c == '.' || c == 'e' || c == 'E' {
					i++
					continue
				}
				if (c == '+' || c == '-') && i > start && (s[i-1] == 'e' || s[i-1] == 'E') {
					i++
					continue
				}
				break
			}
			for k := start; k < i; k++ {
				kinds[k] = jsonKindNumber
			}
		default:
			if lit, ok := matchJSONKeyword(s, i); ok {
				for k := i; k < i+len(lit); k++ {
					kinds[k] = jsonKindKeyword
				}
				i += len(lit)
				continue
			}
			// Skip one rune as plain (keeps highlighter moving on junk).
			_, size := utf8.DecodeRuneInString(s[i:])
			if size < 1 {
				size = 1
			}
			for k := i; k < i+size && k < n; k++ {
				kinds[k] = jsonKindPlain
			}
			i += size
		}
	}
	return kinds
}

func matchJSONKeyword(s string, i int) (string, bool) {
	for _, lit := range []string{"true", "false", "null"} {
		if i+len(lit) <= len(s) && s[i:i+len(lit)] == lit {
			end := i + len(lit)
			if end < len(s) {
				r := rune(s[end])
				if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
					continue
				}
			}
			return lit, true
		}
	}
	return "", false
}

// jsonKindsForRunes maps byte-kinds onto rune indices for editor rendering.
func jsonKindsForRunes(s string) []uint8 {
	byteKinds := jsonTokenKinds(s)
	runes := []rune(s)
	out := make([]uint8, len(runes))
	bi := 0
	for ri := range runes {
		if bi < len(byteKinds) {
			out[ri] = byteKinds[bi]
		}
		size := utf8.RuneLen(runes[ri])
		if size < 1 {
			size = 1
		}
		bi += size
	}
	return out
}
