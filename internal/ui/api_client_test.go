package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseAPIHeaders(t *testing.T) {
	h := parseAPIHeaders("Content-Type: application/json\n# comment\nAccept: text/plain\nBadLine\nX-Empty:\n")
	if got := h.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := h.Get("Accept"); got != "text/plain" {
		t.Fatalf("Accept=%q", got)
	}
	if got := h.Get("X-Empty"); got != "" {
		t.Fatalf("X-Empty=%q want empty", got)
	}
}

func TestApplyAPIAuth(t *testing.T) {
	h := make(http.Header)
	applyAPIAuth(h, apiAuthBearer, " tok ", "", "")
	if got := h.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("bearer=%q", got)
	}

	h = make(http.Header)
	applyAPIAuth(h, apiAuthBasic, "", "alice", "s3cret")
	if !strings.HasPrefix(h.Get("Authorization"), "Basic ") {
		t.Fatalf("basic missing prefix: %q", h.Get("Authorization"))
	}

	h = make(http.Header)
	applyAPIAuth(h, apiAuthNone, "x", "u", "p")
	if h.Get("Authorization") != "" {
		t.Fatalf("none should not set auth")
	}
}

func TestTruncateAPIBody(t *testing.T) {
	if got := truncateAPIBody("abc", 10); got != "abc" {
		t.Fatalf("short=%q", got)
	}
	got := truncateAPIBody(strings.Repeat("x", 20), 10)
	if !strings.HasPrefix(got, strings.Repeat("x", 10)) {
		t.Fatalf("prefix missing")
	}
	if !strings.Contains(got, "truncado") {
		t.Fatalf("missing truncation note: %q", got)
	}
}

func TestMethodAllowsBody(t *testing.T) {
	if !methodAllowsBody("POST") || !methodAllowsBody("put") {
		t.Fatal("POST/PUT should allow body")
	}
	if methodAllowsBody("GET") || methodAllowsBody("HEAD") {
		t.Fatal("GET/HEAD should not allow body")
	}
}

func TestSendAPIRequestGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method=%s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer abc" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		w.Header().Set("X-Test", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	msg := sendAPIRequest(apiRequest{
		Method:   "GET",
		URL:      srv.URL + "/ping",
		Headers:  "Accept: application/json",
		AuthType: apiAuthBearer,
		Token:    "abc",
	})().(apiResponseMsg)

	if msg.err != nil {
		t.Fatalf("err=%v", msg.err)
	}
	if msg.statusCode != 200 {
		t.Fatalf("code=%d", msg.statusCode)
	}
	if !strings.Contains(msg.body, `"ok"`) || !strings.Contains(msg.body, "true") {
		t.Fatalf("body=%q", msg.body)
	}
	if !strings.Contains(msg.headers, "X-Test: 1") {
		t.Fatalf("headers=%q", msg.headers)
	}
	if msg.duration <= 0 || msg.duration > 5*time.Second {
		t.Fatalf("duration=%v", msg.duration)
	}
}
