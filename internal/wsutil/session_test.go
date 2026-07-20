package wsutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNormalizeURL(t *testing.T) {
	got, err := normalizeURL("http://localhost:8080/ws")
	if err != nil || got != "ws://localhost:8080/ws" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	got, err = normalizeURL("example.com/socket")
	if err != nil || got != "ws://example.com/socket" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestDialSendReceive(t *testing.T) {
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		_, msg, err := c.ReadMessage()
		if err != nil {
			return
		}
		_ = c.WriteMessage(websocket.TextMessage, []byte("echo:"+string(msg)))
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	s, err := Dial(wsURL, "Origin: http://localhost\n")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Send("ping"); err != nil {
		t.Fatal(err)
	}
	ev := waitKind(t, s, "message")
	if ev.Text != "echo:ping" {
		t.Fatalf("text=%q", ev.Text)
	}
}

func waitKind(t *testing.T, s *Session, kind string) Event {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %s", kind)
		case ev := <-s.Events():
			if ev.Kind == kind {
				return ev
			}
		}
	}
}
