package jsonutil

import (
	"strings"
	"testing"
)

func TestPrettyMinifyValidate(t *testing.T) {
	raw := `{"b":1,"a":2}`
	pretty, err := Pretty(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pretty, "\n") || !strings.Contains(pretty, `"a"`) {
		t.Fatalf("pretty=%q", pretty)
	}
	mini, err := Minify(pretty)
	if err != nil || mini != `{"b":1,"a":2}` && mini != `{"a":2,"b":1}` {
		// order may vary after parse — just check valid minify
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(mini); err != nil {
			t.Fatal(err)
		}
	}
	if err := Validate(`{"x":`); err == nil {
		t.Fatal("expected validate error")
	} else if !strings.Contains(err.Error(), "linha") {
		t.Fatalf("error should mention line: %v", err)
	}
}

func TestSortKeysAndStripNulls(t *testing.T) {
	out, err := SortKeys(`{"z":1,"a":{"c":2,"b":null}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), `{`) || strings.Index(out, `"a"`) > strings.Index(out, `"z"`) {
		// a should come before z at top level
		ia, iz := strings.Index(out, `"a"`), strings.Index(out, `"z"`)
		if ia < 0 || iz < 0 || ia > iz {
			t.Fatalf("keys not sorted: %s", out)
		}
	}
	stripped, err := StripNulls(`{"a":1,"b":null,"c":[1,null,2]}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stripped, "null") {
		t.Fatalf("nulls remain: %s", stripped)
	}
}

func TestYAMLRoundTrip(t *testing.T) {
	y, err := ToYAML(`{"name":"dev","n":1}`)
	if err != nil {
		t.Fatal(err)
	}
	j, err := FromYAML(y)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(j); err != nil {
		t.Fatal(err)
	}
}

func TestTOMLAndXML(t *testing.T) {
	tomlOut, err := ToTOML(`{"name":"x","port":80}`)
	if err != nil {
		t.Fatal(err)
	}
	j, err := FromTOML(tomlOut)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(j); err != nil {
		t.Fatal(err)
	}
	xmlOut, err := ToXML(`{"hello":"world","n":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(xmlOut, "<hello>") {
		t.Fatalf("xml=%s", xmlOut)
	}
	back, err := FromXML(xmlOut)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(back); err != nil {
		t.Fatal(err)
	}
}

func TestSearchAndDiff(t *testing.T) {
	hits, err := SearchKey(`{"user":{"id":1},"name":"a"}`, "id")
	if err != nil || !strings.Contains(hits, "user.id") {
		t.Fatalf("hits=%q err=%v", hits, err)
	}
	d := DiffText("a\nb\n", "a\nc\n")
	if !strings.Contains(d, "- b") || !strings.Contains(d, "+ c") {
		t.Fatalf("diff=%q", d)
	}
}
