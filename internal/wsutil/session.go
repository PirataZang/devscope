package wsutil

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Event is a lifecycle or traffic event for the TUI.
type Event struct {
	Kind     string // connected | disconnected | message | error
	Text     string
	Inbound  bool
	Binary   bool
	Opcode   int // websocket opcode when known
	ByteSize int
}

// Info holds handshake metadata after a successful Dial.
type Info struct {
	URL           string
	TLS           bool
	Subprotocol   string
	RespStatus    string
	RespHeaders   http.Header
	ReqHeaders    http.Header
	Compression   bool
	ConnectedAt   time.Time
}

// Session is a single WebSocket connection with a read pump.
type Session struct {
	conn   *websocket.Conn
	events chan Event
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	Info   Info
}

// Dial opens a WebSocket to rawURL with optional "Key: value" headers.
func Dial(rawURL, headers string) (*Session, error) {
	u, err := normalizeURL(rawURL)
	if err != nil {
		return nil, err
	}
	hdr := parseHeaders(headers)
	dialer := websocket.Dialer{HandshakeTimeout: 8 * time.Second}
	conn, resp, err := dialer.Dial(u, hdr)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("%v (HTTP %s)", err, resp.Status)
		}
		return nil, err
	}
	info := Info{
		URL:         u,
		TLS:         strings.HasPrefix(u, "wss://"),
		Subprotocol: conn.Subprotocol(),
		ReqHeaders:  hdr.Clone(),
		ConnectedAt: time.Now(),
	}
	if resp != nil {
		info.RespStatus = resp.Status
		info.RespHeaders = resp.Header.Clone()
		ext := strings.ToLower(resp.Header.Get("Sec-WebSocket-Extensions"))
		info.Compression = strings.Contains(ext, "permessage-deflate")
	}
	s := &Session{
		conn:   conn,
		events: make(chan Event, 128),
		done:   make(chan struct{}),
		Info:   info,
	}
	go s.readLoop()
	return s, nil
}

func (s *Session) Events() <-chan Event { return s.events }

func (s *Session) Send(text string) error {
	return s.SendMessage(websocket.TextMessage, []byte(text))
}

func (s *Session) SendBinary(data []byte) error {
	return s.SendMessage(websocket.BinaryMessage, data)
}

func (s *Session) SendMessage(messageType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return fmt.Errorf("não conectado")
	}
	return s.conn.WriteMessage(messageType, data)
}

func (s *Session) Ping() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return fmt.Errorf("não conectado")
	}
	return s.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(3*time.Second))
}

func (s *Session) Close() {
	s.once.Do(func() {
		close(s.done)
		s.mu.Lock()
		if s.conn != nil {
			_ = s.conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			_ = s.conn.Close()
			s.conn = nil
		}
		s.mu.Unlock()
		select {
		case s.events <- Event{Kind: "disconnected", Text: "closed"}:
		default:
		}
	})
}

func (s *Session) readLoop() {
	for {
		select {
		case <-s.done:
			return
		default:
		}
		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			select {
			case <-s.done:
			case s.events <- Event{Kind: "disconnected", Text: err.Error()}:
			default:
			}
			return
		}
		ev := Event{
			Kind:     "message",
			Text:     string(data),
			Inbound:  true,
			Binary:   msgType == websocket.BinaryMessage,
			Opcode:   msgType,
			ByteSize: len(data),
		}
		select {
		case <-s.done:
			return
		case s.events <- ev:
		}
	}
}

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("URL vazia")
	}
	if strings.HasPrefix(raw, "http://") {
		raw = "ws://" + strings.TrimPrefix(raw, "http://")
	}
	if strings.HasPrefix(raw, "https://") {
		raw = "wss://" + strings.TrimPrefix(raw, "https://")
	}
	if !strings.HasPrefix(raw, "ws://") && !strings.HasPrefix(raw, "wss://") {
		raw = "ws://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("URL inválida")
	}
	return u.String(), nil
}

func parseHeaders(raw string) http.Header {
	h := http.Header{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, ':')
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if key != "" {
			h.Add(key, val)
		}
	}
	return h
}

// FormatHandshake renders request/response headers for the inspector.
func FormatHandshake(info Info) string {
	var b strings.Builder
	b.WriteString("REQUEST\n")
	b.WriteString("GET " + pathOf(info.URL) + " HTTP/1.1\n")
	for k, vals := range info.ReqHeaders {
		for _, v := range vals {
			b.WriteString(k + ": " + v + "\n")
		}
	}
	b.WriteString("\nRESPONSE\n")
	if info.RespStatus != "" {
		b.WriteString(info.RespStatus + "\n")
	}
	for k, vals := range info.RespHeaders {
		for _, v := range vals {
			b.WriteString(k + ": " + v + "\n")
		}
	}
	if info.Subprotocol != "" {
		b.WriteString("Sec-WebSocket-Protocol: " + info.Subprotocol + "\n")
	}
	return b.String()
}

func pathOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.RequestURI() == "" {
		return "/ws"
	}
	return u.RequestURI()
}
