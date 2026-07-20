package jwtutil

import (
	"strings"
	"testing"
)

func TestSignVerifyDecode(t *testing.T) {
	claims := `{"sub":"u1","name":"igor"}`
	tok, err := Sign(claims, "secret", "HS256")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token=%q", tok)
	}
	out, err := Verify(tok, "secret", "HS256")
	if err != nil || !strings.Contains(out, "VALID") {
		t.Fatalf("verify: %v out=%q", err, out)
	}
	if _, err := Verify(tok, "wrong", "HS256"); err == nil {
		t.Fatal("expected verify fail")
	}
	pretty, err := DecodePretty(tok)
	if err != nil || !strings.Contains(pretty, "PAYLOAD") {
		t.Fatalf("decode: %v %q", err, pretty)
	}
	exp, err := ExportJSON(tok)
	if err != nil || !strings.Contains(exp, `"header"`) {
		t.Fatalf("export: %v %q", err, exp)
	}
	cj, err := ClaimsJSON(tok)
	if err != nil || !strings.Contains(cj, "igor") {
		t.Fatalf("claims: %v %q", err, cj)
	}
}

func TestGenerateClaims(t *testing.T) {
	c := GenerateClaims()
	if !strings.Contains(c, "sub") || !strings.Contains(c, "exp") {
		t.Fatalf("%q", c)
	}
	tok, err := Sign(c, "s", "HS256")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(tok, "s", "HS256"); err != nil {
		t.Fatal(err)
	}
}
