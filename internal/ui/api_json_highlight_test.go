package ui

import (
	"strings"
	"testing"
)

func TestJSONTokenKindsKeyVsString(t *testing.T) {
	s := `{"name": "igor", "n": 1, "ok": true, "x": null}`
	kinds := jsonTokenKinds(s)

	// "name" is a key
	nameStart := strings.Index(s, `"name"`)
	if kinds[nameStart] != jsonKindKey {
		t.Fatalf("name key kind=%d", kinds[nameStart])
	}
	// value "igor"
	igorStart := strings.Index(s, `"igor"`)
	if kinds[igorStart] != jsonKindString {
		t.Fatalf("igor string kind=%d", kinds[igorStart])
	}
	// braces
	if kinds[0] != jsonKindPunct || kinds[len(s)-1] != jsonKindPunct {
		t.Fatalf("braces should be punct")
	}
	// number
	nPos := strings.Index(s, "1")
	if kinds[nPos] != jsonKindNumber {
		t.Fatalf("number kind=%d", kinds[nPos])
	}
	truePos := strings.Index(s, "true")
	if kinds[truePos] != jsonKindKeyword {
		t.Fatalf("true kind=%d", kinds[truePos])
	}
}

func TestJSONHighlightRendersANSI(t *testing.T) {
	line := `{"name": "x"}`
	got := renderJSONColumns(line, 0, 40)
	if got == line {
		t.Fatal("expected ANSI styling")
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, `"name"`) || !strings.Contains(plain, `"x"`) {
		t.Fatalf("lost text: %q", plain)
	}
}
