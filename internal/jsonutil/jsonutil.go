package jsonutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Validate returns nil or an error with line/column when possible.
func Validate(raw string) error {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return formatJSONError(err, raw)
	}
	if dec.More() {
		return fmt.Errorf("JSON inválido: conteúdo extra após o valor")
	}
	return nil
}

func formatJSONError(err error, raw string) error {
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		line, col := offsetLineCol(raw, int(syn.Offset))
		return fmt.Errorf("JSON inválido na linha %d, coluna %d: %s", line, col, syn.Error())
	}
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		line, col := offsetLineCol(raw, int(ute.Offset))
		return fmt.Errorf("tipo inválido na linha %d, coluna %d: %s", line, col, ute.Error())
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "EOF") {
		line, col := offsetLineCol(raw, len(raw))
		return fmt.Errorf("JSON inválido na linha %d, coluna %d: %v", line, col, err)
	}
	return fmt.Errorf("JSON inválido: %w", err)
}

func offsetLineCol(s string, offset int) (line, col int) {
	line, col = 1, 1
	if offset < 0 {
		offset = 0
	}
	if offset > len(s) {
		offset = len(s)
	}
	for i := 0; i < offset; i++ {
		if s[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

func parse(raw string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, formatJSONError(err, raw)
	}
	return v, nil
}

// Pretty returns indented JSON.
func Pretty(raw string) (string, error) {
	v, err := parse(raw)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// Minify returns compact JSON.
func Minify(raw string) (string, error) {
	v, err := parse(raw)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SortKeys returns JSON with object keys sorted recursively.
func SortKeys(raw string) (string, error) {
	v, err := parse(raw)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(sortValue(v), "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

func sortValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(keys))
		for _, k := range keys {
			out[k] = sortValue(t[k])
		}
		return out
	case []any:
		for i := range t {
			t[i] = sortValue(t[i])
		}
		return t
	default:
		return v
	}
}

// StripNulls removes null fields from objects (and null array elements).
func StripNulls(raw string) (string, error) {
	v, err := parse(raw)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(stripNulls(v), "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

func stripNulls(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if val == nil {
				continue
			}
			out[k] = stripNulls(val)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, val := range t {
			if val == nil {
				continue
			}
			out = append(out, stripNulls(val))
		}
		return out
	default:
		return v
	}
}

// ToYAML converts JSON text to YAML.
func ToYAML(raw string) (string, error) {
	v, err := parse(raw)
	if err != nil {
		return "", err
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FromYAML converts YAML text to pretty JSON.
func FromYAML(raw string) (string, error) {
	var v any
	if err := yaml.Unmarshal([]byte(raw), &v); err != nil {
		return "", fmt.Errorf("YAML inválido: %w", err)
	}
	b, err := json.MarshalIndent(normalizeYAML(v), "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

func normalizeYAML(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeYAML(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = normalizeYAML(val)
		}
		return out
	case []any:
		for i := range t {
			t[i] = normalizeYAML(t[i])
		}
		return t
	default:
		return v
	}
}

// ToTOML converts JSON object to TOML.
func ToTOML(raw string) (string, error) {
	v, err := parse(raw)
	if err != nil {
		return "", err
	}
	b, err := toml.Marshal(plainNumbers(v))
	if err != nil {
		return "", fmt.Errorf("TOML: %w", err)
	}
	return string(b), nil
}

func plainNumbers(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = plainNumbers(val)
		}
		return out
	case []any:
		for i := range t {
			t[i] = plainNumbers(t[i])
		}
		return t
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	default:
		return v
	}
}

// FromTOML converts TOML to pretty JSON.
func FromTOML(raw string) (string, error) {
	var v any
	if err := toml.Unmarshal([]byte(raw), &v); err != nil {
		return "", fmt.Errorf("TOML inválido: %w", err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// ToXML converts JSON to a simple XML document.
func ToXML(raw string) (string, error) {
	v, err := parse(raw)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	writeXML(&b, "root", v, 0)
	return b.String(), nil
}

// FromXML converts simple XML (from ToXML / flat-ish) to JSON via yaml-like parse of text nodes.
// For MVP we parse with encoding/xml into a generic structure is hard; use a tiny recursive tag parser.
func FromXML(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, `<?xml version="1.0" encoding="UTF-8"?>`)
	raw = strings.TrimSpace(raw)
	v, _, err := parseXMLElement([]rune(raw), 0)
	if err != nil {
		return "", fmt.Errorf("XML inválido: %w", err)
	}
	// unwrap single root
	if m, ok := v.(map[string]any); ok && len(m) == 1 {
		if root, ok := m["root"]; ok {
			v = root
		}
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

func writeXML(b *strings.Builder, tag string, v any, indent int) {
	pad := strings.Repeat("  ", indent)
	switch t := v.(type) {
	case map[string]any:
		b.WriteString(pad + "<" + tag + ">\n")
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			writeXML(b, sanitizeXMLTag(k), t[k], indent+1)
		}
		b.WriteString(pad + "</" + tag + ">\n")
	case []any:
		for _, item := range t {
			writeXML(b, tag, item, indent)
		}
	case nil:
		b.WriteString(pad + "<" + tag + " nil=\"true\"/>\n")
	default:
		b.WriteString(pad + "<" + tag + ">" + escapeXML(fmt.Sprint(t)) + "</" + tag + ">\n")
	}
}

func sanitizeXMLTag(s string) string {
	if s == "" {
		return "item"
	}
	var b strings.Builder
	for i, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' ||
			(i > 0 && ((r >= '0' && r <= '9') || r == '-' || r == '.'))
		if ok {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || (out[0] >= '0' && out[0] <= '9') {
		return "n_" + out
	}
	return out
}

func escapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

func parseXMLElement(runes []rune, i int) (any, int, error) {
	for i < len(runes) && (runes[i] == ' ' || runes[i] == '\n' || runes[i] == '\t' || runes[i] == '\r') {
		i++
	}
	if i >= len(runes) || runes[i] != '<' {
		return nil, i, fmt.Errorf("esperado '<'")
	}
	i++
	if i < len(runes) && runes[i] == '/' {
		return nil, i, fmt.Errorf("tag de fechamento inesperada")
	}
	start := i
	for i < len(runes) && runes[i] != '>' && runes[i] != ' ' && runes[i] != '/' {
		i++
	}
	tag := string(runes[start:i])
	for i < len(runes) && runes[i] != '>' && runes[i] != '/' {
		i++
	}
	selfClose := i < len(runes) && runes[i] == '/'
	if selfClose {
		i++
	}
	if i >= len(runes) || runes[i] != '>' {
		return nil, i, fmt.Errorf("tag malformada")
	}
	i++
	if selfClose {
		return map[string]any{tag: nil}, i, nil
	}

	var text strings.Builder
	children := map[string]any{}
	order := []string{}
	for {
		for i < len(runes) && (runes[i] == ' ' || runes[i] == '\n' || runes[i] == '\t' || runes[i] == '\r') {
			i++
		}
		if i >= len(runes) {
			return nil, i, fmt.Errorf("XML incompleto")
		}
		if runes[i] == '<' {
			if i+1 < len(runes) && runes[i+1] == '/' {
				i += 2
				end := i
				for i < len(runes) && runes[i] != '>' {
					i++
				}
				closeTag := string(runes[end:i])
				if i < len(runes) {
					i++
				}
				if closeTag != tag {
					return nil, i, fmt.Errorf("fecha </%s> esperado </%s>", closeTag, tag)
				}
				break
			}
			child, ni, err := parseXMLElement(runes, i)
			if err != nil {
				return nil, ni, err
			}
			i = ni
			cm, ok := child.(map[string]any)
			if !ok || len(cm) != 1 {
				continue
			}
			for k, val := range cm {
				if prev, exists := children[k]; exists {
					if arr, ok := prev.([]any); ok {
						children[k] = append(arr, val)
					} else {
						children[k] = []any{prev, val}
					}
				} else {
					children[k] = val
					order = append(order, k)
				}
			}
			continue
		}
		for i < len(runes) && runes[i] != '<' {
			text.WriteRune(runes[i])
			i++
		}
	}
	txt := strings.TrimSpace(text.String())
	if len(children) == 0 {
		if txt == "" {
			return map[string]any{tag: nil}, i, nil
		}
		return map[string]any{tag: coerceScalar(txt)}, i, nil
	}
	_ = order
	return map[string]any{tag: children}, i, nil
}

func coerceScalar(s string) any {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return json.Number(strconv.FormatInt(n, 10))
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// DetectFormat guesses json/yaml/toml/xml from content.
func DetectFormat(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "empty"
	}
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return "json"
	}
	if strings.HasPrefix(s, "<?xml") || (strings.HasPrefix(s, "<") && strings.Contains(s, ">")) {
		return "xml"
	}
	if err := Validate(s); err == nil {
		return "json"
	}
	var y any
	if yaml.Unmarshal([]byte(s), &y) == nil && !strings.Contains(s, "=") {
		// weak: many toml also parse as yaml; prefer toml if looks like key=
		if looksLikeTOML(s) {
			return "toml"
		}
		return "yaml"
	}
	if looksLikeTOML(s) {
		return "toml"
	}
	return "text"
}

func looksLikeTOML(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			return true
		}
		if i := strings.IndexByte(line, '='); i > 0 {
			return true
		}
		break
	}
	return false
}

// ConvertToggle JSON⇄format for the named format (yaml|toml|xml).
func ConvertToggle(raw, format string) (string, string, error) {
	format = strings.ToLower(format)
	src := DetectFormat(raw)
	switch format {
	case "yaml":
		if src == "yaml" {
			out, err := FromYAML(raw)
			return out, "YAML → JSON", err
		}
		out, err := ToYAML(raw)
		return out, "JSON → YAML", err
	case "toml":
		if src == "toml" {
			out, err := FromTOML(raw)
			return out, "TOML → JSON", err
		}
		out, err := ToTOML(raw)
		return out, "JSON → TOML", err
	case "xml":
		if src == "xml" {
			out, err := FromXML(raw)
			return out, "XML → JSON", err
		}
		out, err := ToXML(raw)
		return out, "JSON → XML", err
	default:
		return "", "", fmt.Errorf("formato desconhecido: %s", format)
	}
}

// SearchKey finds paths whose key equals query (case-insensitive contains).
func SearchKey(raw, query string) (string, error) {
	v, err := parse(raw)
	if err != nil {
		return "", err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return "", fmt.Errorf("informe a chave")
	}
	var hits []string
	searchWalk(v, "", query, &hits)
	if len(hits) == 0 {
		return "(nenhum match)\n", nil
	}
	return strings.Join(hits, "\n") + "\n", nil
}

func searchWalk(v any, path, query string, hits *[]string) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := path + "." + k
			if path == "" {
				p = k
			}
			if strings.Contains(strings.ToLower(k), query) {
				b, _ := json.Marshal(t[k])
				*hits = append(*hits, p+" = "+string(b))
			}
			searchWalk(t[k], p, query, hits)
		}
	case []any:
		for i, item := range t {
			searchWalk(item, path+"["+strconv.Itoa(i)+"]", query, hits)
		}
	}
}

// DiffText is a simple unified line diff (a = left/input, b = right/output).
func DiffText(a, b string) string {
	al := strings.Split(strings.ReplaceAll(a, "\r\n", "\n"), "\n")
	bl := strings.Split(strings.ReplaceAll(b, "\r\n", "\n"), "\n")
	// trim trailing empty from split
	if len(al) > 0 && al[len(al)-1] == "" {
		al = al[:len(al)-1]
	}
	if len(bl) > 0 && bl[len(bl)-1] == "" {
		bl = bl[:len(bl)-1]
	}
	var out bytes.Buffer
	out.WriteString("--- input\n+++ output\n")
	i, j := 0, 0
	for i < len(al) || j < len(bl) {
		if i < len(al) && j < len(bl) && al[i] == bl[j] {
			out.WriteString("  " + al[i] + "\n")
			i++
			j++
			continue
		}
		// look ahead small window for sync
		if j < len(bl) && (i >= len(al) || !lineIn(bl[j], al[i:min(i+8, len(al))])) {
			if i < len(al) && (j >= len(bl) || !lineIn(al[i], bl[j:min(j+8, len(bl))])) {
				out.WriteString("- " + al[i] + "\n")
				i++
				continue
			}
			out.WriteString("+ " + bl[j] + "\n")
			j++
			continue
		}
		if i < len(al) {
			out.WriteString("- " + al[i] + "\n")
			i++
			continue
		}
		if j < len(bl) {
			out.WriteString("+ " + bl[j] + "\n")
			j++
		}
	}
	return out.String()
}

func lineIn(line string, lines []string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
