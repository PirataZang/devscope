package jwtutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
	"time"
)

// Decode splits and parses header + payload without verifying the signature.
func Decode(token string) (header, payload map[string]any, err error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, nil, fmt.Errorf("JWT inválido: esperado 3 partes (header.payload.signature)")
	}
	header, err = decodePart(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("header: %w", err)
	}
	payload, err = decodePart(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("payload: %w", err)
	}
	return header, payload, nil
}

func decodePart(seg string) (map[string]any, error) {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		// some tokens include padding
		b, err = base64.URLEncoding.DecodeString(seg)
		if err != nil {
			return nil, err
		}
	}
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// DecodePretty returns a human-readable decode (header, payload, times).
func DecodePretty(token string) (string, error) {
	header, payload, err := Decode(token)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("HEADER\n")
	b.WriteString(mustIndent(header))
	b.WriteString("\nPAYLOAD\n")
	b.WriteString(mustIndent(payload))
	b.WriteString(formatTimeHints(payload))
	if alg, _ := header["alg"].(string); alg != "" {
		b.WriteString("\nALG  " + alg + "\n")
	}
	return b.String(), nil
}

func formatTimeHints(payload map[string]any) string {
	var lines []string
	for _, key := range []string{"iat", "nbf", "exp"} {
		v, ok := payload[key]
		if !ok {
			continue
		}
		sec, ok := jsonNumberSeconds(v)
		if !ok {
			continue
		}
		t := time.Unix(sec, 0).Local()
		extra := ""
		if key == "exp" {
			if time.Now().After(t) {
				extra = "  (expirado)"
			} else {
				extra = "  (em " + time.Until(t).Round(time.Second).String() + ")"
			}
		}
		lines = append(lines, fmt.Sprintf("%s  %d  →  %s%s", key, sec, t.Format(time.RFC3339), extra))
	}
	if len(lines) == 0 {
		return ""
	}
	return "\nTIMES\n" + strings.Join(lines, "\n") + "\n"
}

func jsonNumberSeconds(v any) (int64, bool) {
	switch t := v.(type) {
	case json.Number:
		i, err := t.Int64()
		return i, err == nil
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	default:
		return 0, false
	}
}

func mustIndent(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(b) + "\n"
}

// ExportJSON returns {"header":...,"payload":...}.
func ExportJSON(token string) (string, error) {
	header, payload, err := Decode(token)
	if err != nil {
		return "", err
	}
	out := map[string]any{"header": header, "payload": payload}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// ClaimsJSON returns only the payload as pretty JSON.
func ClaimsJSON(token string) (string, error) {
	_, payload, err := Decode(token)
	if err != nil {
		return "", err
	}
	return mustIndent(payload), nil
}

// Verify checks HMAC signature (and reports exp if present).
func Verify(token, secret, alg string) (string, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("JWT inválido: esperado 3 partes")
	}
	header, payload, err := Decode(token)
	if err != nil {
		return "", err
	}
	tokenAlg, _ := header["alg"].(string)
	if alg == "" {
		alg = tokenAlg
	}
	if tokenAlg != "" && !strings.EqualFold(tokenAlg, alg) {
		return "", fmt.Errorf("alg do token %q ≠ selecionado %q", tokenAlg, alg)
	}
	mac, err := signHMAC(parts[0]+"."+parts[1], secret, alg)
	if err != nil {
		return "", err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		sig, err = base64.URLEncoding.DecodeString(parts[2])
		if err != nil {
			return "", fmt.Errorf("signature: %w", err)
		}
	}
	if !hmac.Equal(mac, sig) {
		return "", fmt.Errorf("assinatura inválida")
	}
	status := "VALID ✓  assinatura OK"
	if sec, ok := jsonNumberSeconds(payload["exp"]); ok && time.Now().After(time.Unix(sec, 0)) {
		status += "  ·  token expirado"
	}
	pretty, _ := DecodePretty(token)
	return status + "\n\n" + pretty, nil
}

// Sign creates a JWT from claims JSON using HMAC alg.
func Sign(claimsJSON, secret, alg string) (string, error) {
	var claims map[string]any
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(claimsJSON)))
	dec.UseNumber()
	if err := dec.Decode(&claims); err != nil {
		return "", fmt.Errorf("claims JSON inválido: %w", err)
	}
	if alg == "" {
		alg = "HS256"
	}
	header := map[string]any{"alg": alg, "typ": "JWT"}
	hSeg, err := encodePart(header)
	if err != nil {
		return "", err
	}
	pSeg, err := encodePart(claims)
	if err != nil {
		return "", err
	}
	signing := hSeg + "." + pSeg
	mac, err := signHMAC(signing, secret, alg)
	if err != nil {
		return "", err
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(mac), nil
}

func encodePart(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func signHMAC(signingInput, secret, alg string) ([]byte, error) {
	var h func() hash.Hash
	switch strings.ToUpper(alg) {
	case "HS256":
		h = sha256.New
	case "HS384":
		h = sha512.New384
	case "HS512":
		h = sha512.New
	default:
		return nil, fmt.Errorf("alg não suportado: %s (use HS256/HS384/HS512)", alg)
	}
	if secret == "" {
		return nil, fmt.Errorf("secret vazio")
	}
	mac := hmac.New(h, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	return mac.Sum(nil), nil
}

// GenerateClaims returns a starter claims payload.
func GenerateClaims() string {
	now := time.Now()
	claims := map[string]any{
		"sub":  "1234567890",
		"name": "DevScope",
		"iat":  now.Unix(),
		"exp":  now.Add(1 * time.Hour).Unix(),
	}
	return mustIndent(claims)
}

// Algs lists supported HMAC algorithms.
func Algs() []string {
	return []string{"HS256", "HS384", "HS512"}
}
