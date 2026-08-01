package collectors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MergeComposeYAML merges services from yamlText into the project's compose file.
// yamlText may be a full compose doc (`services:`) or a single service map (`name: ...`).
func MergeComposeYAML(projectPath, yamlText string) (composePath string, err error) {
	yamlText = strings.TrimSpace(yamlText)
	if yamlText == "" {
		return "", fmt.Errorf("YAML vazio")
	}
	incoming, err := parseComposeServices(yamlText)
	if err != nil {
		return "", err
	}
	if len(incoming) == 0 {
		return "", fmt.Errorf("nenhum serviço no YAML")
	}

	composePath = ComposeFile(projectPath)
	doc := map[string]any{}
	if composePath == "" {
		composePath = filepath.Join(projectPath, "docker-compose.yml")
		doc["services"] = map[string]any{}
	} else {
		raw, readErr := os.ReadFile(composePath)
		if readErr != nil {
			return "", readErr
		}
		if strings.TrimSpace(string(raw)) != "" {
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				return "", fmt.Errorf("compose inválido: %w", err)
			}
		}
	}

	services := ensureStringMap(doc, "services")
	for name, cfg := range incoming {
		services[name] = cfg
	}
	doc["services"] = services

	if volIn, err := parseComposeTopLevelVolumes(yamlText); err == nil && len(volIn) > 0 {
		volumes := ensureStringMap(doc, "volumes")
		for name, cfg := range volIn {
			if _, exists := volumes[name]; !exists {
				volumes[name] = cfg
			}
		}
		doc["volumes"] = volumes
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(composePath, out, 0o644); err != nil {
		return "", err
	}
	return composePath, nil
}

func parseComposeServices(yamlText string) (map[string]any, error) {
	var root map[string]any
	if err := yaml.Unmarshal([]byte(yamlText), &root); err != nil {
		return nil, fmt.Errorf("YAML inválido: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("YAML vazio")
	}
	if raw, ok := root["services"]; ok {
		return asStringMap(raw)
	}
	// Treat root as a single-level services map (name -> config).
	return asStringMap(root)
}

func asStringMap(v any) (map[string]any, error) {
	switch m := v.(type) {
	case map[string]any:
		return m, nil
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, nil
	default:
		return nil, fmt.Errorf("esperado mapa de serviços")
	}
}

// ComposeServiceTemplate returns a starter YAML snippet for manual or hub-based add.
// Prefer BuildComposeServiceYAML when the caller needs the source label.
func ComposeServiceTemplate(image string) string {
	yamlText, _ := BuildComposeServiceYAML(image)
	return yamlText
}

func ensureStringMap(doc map[string]any, key string) map[string]any {
	if m, ok := doc[key].(map[string]any); ok && m != nil {
		return m
	}
	if raw, ok := doc[key].(map[any]any); ok {
		out := map[string]any{}
		for k, v := range raw {
			out[fmt.Sprint(k)] = v
		}
		return out
	}
	return map[string]any{}
}

func parseComposeTopLevelVolumes(yamlText string) (map[string]any, error) {
	var root map[string]any
	if err := yaml.Unmarshal([]byte(yamlText), &root); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, nil
	}
	raw, ok := root["volumes"]
	if !ok || raw == nil {
		return nil, nil
	}
	return asStringMap(raw)
}

func serviceNameFromImage(image string) string {
	image = strings.TrimPrefix(image, "library/")
	if i := strings.LastIndex(image, "/"); i >= 0 {
		image = image[i+1:]
	}
	if i := strings.Index(image, ":"); i >= 0 {
		image = image[:i]
	}
	image = strings.ToLower(image)
	var b strings.Builder
	for _, r := range image {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-_")
	if name == "" {
		return "service"
	}
	return name
}
