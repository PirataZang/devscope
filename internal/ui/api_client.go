package ui

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Soft cap only to protect the TUI — large enough for real API payloads.
const apiResponseBodyLimit = 8 * 1024 * 1024

type apiResponseMsg struct {
	status     string
	statusCode int
	duration   time.Duration
	headers    string
	body       string
	err        error
	method     string
	url        string
}

type apiRequest struct {
	Method   string
	URL      string
	Headers  string
	AuthType apiAuthType
	Token    string
	User     string
	Pass     string
	Body     string
}

func parseAPIHeaders(text string) http.Header {
	h := make(http.Header)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}
		h.Add(key, val)
	}
	return h
}

func applyAPIAuth(h http.Header, authType apiAuthType, token, user, pass string) {
	switch authType {
	case apiAuthBearer:
		token = strings.TrimSpace(token)
		if token != "" {
			h.Set("Authorization", "Bearer "+token)
		}
	case apiAuthBasic:
		user = strings.TrimSpace(user)
		if user != "" || pass != "" {
			raw := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
			h.Set("Authorization", "Basic "+raw)
		}
	}
}

func methodAllowsBody(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func truncateAPIBody(body string, limit int) string {
	if limit <= 0 || len(body) <= limit {
		return body
	}
	return body[:limit] + fmt.Sprintf("\n\n… truncado (%d bytes total)", len(body))
}

func formatAPIResponseHeaders(h http.Header) string {
	if len(h) == 0 {
		return "(sem headers)"
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	var b strings.Builder
	for _, k := range keys {
		for _, v := range h.Values(k) {
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func newAPIHTTPClient() *http.Client {
	transport := http.DefaultTransport
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = t.Clone()
	}
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("parado após 10 redirects")
			}
			return nil
		},
	}
}

func sendAPIRequest(req apiRequest) tea.Cmd {
	return func() tea.Msg {
		method := strings.ToUpper(strings.TrimSpace(req.Method))
		if method == "" {
			method = http.MethodGet
		}
		url := strings.TrimSpace(req.URL)
		url = strings.Trim(url, "\"'")
		if url == "" {
			return apiResponseMsg{err: fmt.Errorf("URL vazia"), method: method, url: url}
		}
		if !strings.Contains(url, "://") {
			url = "https://" + url
		}

		var bodyReader io.Reader
		if methodAllowsBody(method) && strings.TrimSpace(req.Body) != "" {
			bodyReader = strings.NewReader(req.Body)
		}

		httpReq, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return apiResponseMsg{err: err, method: method, url: url}
		}

		// Merge headers onto the request (don't wipe defaults).
		headers := parseAPIHeaders(req.Headers)
		applyAPIAuth(headers, req.AuthType, req.Token, req.User, req.Pass)
		for key, vals := range headers {
			for i, v := range vals {
				if i == 0 {
					httpReq.Header.Set(key, v)
				} else {
					httpReq.Header.Add(key, v)
				}
			}
		}
		if httpReq.Header.Get("User-Agent") == "" {
			httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DevScope/1.0)")
		}
		if methodAllowsBody(method) && bodyReader != nil && httpReq.Header.Get("Content-Type") == "" {
			httpReq.Header.Set("Content-Type", "application/json")
		}

		client := newAPIHTTPClient()
		start := time.Now()
		resp, err := client.Do(httpReq)
		duration := time.Since(start)
		if err != nil {
			return apiResponseMsg{err: fmt.Errorf("%w", explainAPINetError(err)), duration: duration, method: method, url: url}
		}
		defer resp.Body.Close()

		limited := io.LimitReader(resp.Body, apiResponseBodyLimit+1)
		raw, readErr := io.ReadAll(limited)
		body := string(raw)
		if len(raw) > apiResponseBodyLimit {
			body = truncateAPIBody(body, apiResponseBodyLimit)
		}
		if readErr != nil && body == "" {
			return apiResponseMsg{
				err:        readErr,
				duration:   duration,
				status:     resp.Status,
				statusCode: resp.StatusCode,
				headers:    formatAPIResponseHeaders(resp.Header),
				method:     method,
				url:        url,
			}
		}

		return apiResponseMsg{
			status:     resp.Status,
			statusCode: resp.StatusCode,
			duration:   duration,
			headers:    formatAPIResponseHeaders(resp.Header),
			body:       maybePrettyJSON(body),
			method:     method,
			url:        url,
		}
	}
}

func explainAPINetError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection reset by peer"),
		strings.Contains(msg, "conexão reiniciada"),
		strings.Contains(msg, "Connexion réinitialisée"):
		return fmt.Errorf("%v\n\ndica: a rede/firewall fechou a conexão TLS com esse host (não é bloqueio do DevScope). Teste https://httpbin.org/get ou outra API acessível da sua rede", err)
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "Timeout"), strings.Contains(msg, "deadline exceeded"):
		return fmt.Errorf("%v\n\ndica: timeout — host lento ou inacessível da sua rede", err)
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "lookup"):
		return fmt.Errorf("%v\n\ndica: DNS não resolveu o host", err)
	default:
		return err
	}
}

func maybePrettyJSON(body string) string {
	trim := strings.TrimSpace(body)
	if trim == "" || (trim[0] != '{' && trim[0] != '[') {
		return body
	}
	var v any
	if err := json.Unmarshal([]byte(trim), &v); err != nil {
		return body
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return body
	}
	return string(out)
}

func wrapAPIErrorLines(errText string, width int) []string {
	errText = strings.TrimSpace(errText)
	if errText == "" {
		return nil
	}
	prefix := "erro: "
	text := prefix + errText
	if width <= 8 {
		return []string{StyleUnhealthy.Render(text)}
	}
	var lines []string
	runes := []rune(text)
	for len(runes) > 0 {
		n := width
		if n > len(runes) {
			n = len(runes)
		}
		// prefer break on space
		chunk := runes[:n]
		if n < len(runes) {
			if sp := strings.LastIndex(string(chunk), " "); sp > width/3 {
				n = sp + 1
				chunk = runes[:n]
			}
		}
		lines = append(lines, StyleUnhealthy.Render(string(chunk)))
		runes = runes[n:]
	}
	return lines
}
