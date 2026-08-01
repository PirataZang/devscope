package collectors

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ComposeSourcePreset   = "preset"
	ComposeSourceManifest = "manifesto"
	ComposeSourceMinimal  = "mínimo"
	ComposeSourceManual   = "manual"
)

type composePreset struct {
	keys        []string
	ports       []string // "host:container"
	env         map[string]string
	volumes     []string // "volname:path" or "./:/app" bind mounts
	workingDir  string
	command     string // YAML scalar/list after "command: "
	healthcheck string // lines under healthcheck (no leading indent)
}

var composePresets = []composePreset{
	{
		keys:  []string{"postgres", "postgresql"},
		ports: []string{"5432:5432"},
		env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "changeme",
			"POSTGRES_DB":       "app",
		},
		volumes:     []string{"postgres_data:/var/lib/postgresql/data"},
		healthcheck: "test: [\"CMD-SHELL\", \"pg_isready -U $$POSTGRES_USER\"]\ninterval: 5s\ntimeout: 5s\nretries: 5",
	},
	{
		keys:  []string{"mysql"},
		ports: []string{"3306:3306"},
		env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "changeme",
			"MYSQL_DATABASE":      "app",
			"MYSQL_USER":          "app",
			"MYSQL_PASSWORD":      "changeme",
		},
		volumes:     []string{"mysql_data:/var/lib/mysql"},
		healthcheck: "test: [\"CMD\", \"mysqladmin\", \"ping\", \"-h\", \"127.0.0.1\", \"-uroot\", \"-p$$MYSQL_ROOT_PASSWORD\"]\ninterval: 5s\ntimeout: 5s\nretries: 10",
	},
	{
		keys:  []string{"mariadb"},
		ports: []string{"3306:3306"},
		env: map[string]string{
			"MARIADB_ROOT_PASSWORD": "changeme",
			"MARIADB_DATABASE":      "app",
			"MARIADB_USER":          "app",
			"MARIADB_PASSWORD":      "changeme",
		},
		volumes: []string{"mariadb_data:/var/lib/mysql"},
	},
	{
		keys:  []string{"mongo", "mongodb"},
		ports: []string{"27017:27017"},
		env: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": "root",
			"MONGO_INITDB_ROOT_PASSWORD": "changeme",
		},
		volumes: []string{"mongo_data:/data/db"},
	},
	{
		keys:        []string{"redis"},
		ports:       []string{"6379:6379"},
		volumes:     []string{"redis_data:/data"},
		healthcheck: "test: [\"CMD\", \"redis-cli\", \"ping\"]\ninterval: 5s\ntimeout: 3s\nretries: 5",
	},
	{
		keys:  []string{"nginx"},
		ports: []string{"8080:80"},
	},
	{
		keys:  []string{"traefik"},
		ports: []string{"80:80", "8080:8080"},
	},
	{
		keys:  []string{"rabbitmq"},
		ports: []string{"5672:5672", "15672:15672"},
		env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "guest",
			"RABBITMQ_DEFAULT_PASS": "changeme",
		},
		volumes: []string{"rabbitmq_data:/var/lib/rabbitmq"},
	},
	{
		keys:  []string{"elasticsearch"},
		ports: []string{"9200:9200", "9300:9300"},
		env: map[string]string{
			"discovery.type":         "single-node",
			"xpack.security.enabled": "false",
			"ES_JAVA_OPTS":           "-Xms512m -Xmx512m",
		},
		volumes: []string{"elasticsearch_data:/usr/share/elasticsearch/data"},
	},
	{
		keys:  []string{"adminer"},
		ports: []string{"8080:8080"},
	},
	// Language runtimes — official images have almost no ExposedPorts; seed a real app skeleton.
	{
		keys:       []string{"node"},
		ports:      []string{"3000:3000"},
		workingDir: "/app",
		command:    `["npm", "run", "dev"]`,
		env: map[string]string{
			"NODE_ENV": "development",
			"PORT":     "3000",
		},
		volumes: []string{
			"./:/app",
			"node_modules:/app/node_modules",
		},
	},
	{
		keys:       []string{"bun"},
		ports:      []string{"3000:3000"},
		workingDir: "/app",
		command:    `["bun", "run", "dev"]`,
		env: map[string]string{
			"NODE_ENV": "development",
			"PORT":     "3000",
		},
		volumes: []string{"./:/app"},
	},
	{
		keys:       []string{"python"},
		ports:      []string{"8000:8000"},
		workingDir: "/app",
		command:    `["python", "-m", "http.server", "8000"]`,
		env: map[string]string{
			"PYTHONUNBUFFERED": "1",
		},
		volumes: []string{"./:/app"},
	},
	{
		keys:       []string{"golang", "go"},
		ports:      []string{"8080:8080"},
		workingDir: "/app",
		command:    `["go", "run", "."]`,
		env: map[string]string{
			"CGO_ENABLED": "0",
		},
		volumes: []string{"./:/app"},
	},
	{
		keys:       []string{"php"},
		ports:      []string{"8080:80"},
		workingDir: "/var/www/html",
		volumes:    []string{"./:/var/www/html"},
	},
	{
		keys:       []string{"ruby"},
		ports:      []string{"3000:3000"},
		workingDir: "/app",
		command:    `["bash", "-lc", "bundle install && rails server -b 0.0.0.0 -p 3000"]`,
		env: map[string]string{
			"RAILS_ENV": "development",
		},
		volumes: []string{"./:/app"},
	},
}

// MatchComposePreset returns a known preset for the image base name.
func MatchComposePreset(image string) (composePreset, bool) {
	base := composeImageBaseName(image)
	if base == "" {
		return composePreset{}, false
	}
	for _, p := range composePresets {
		for _, k := range p.keys {
			if base == k {
				return p, true
			}
		}
	}
	// also match trailing segment aliases already covered; bitnami/postgresql → postgresql
	return composePreset{}, false
}

// BuildComposeServiceYAML builds a realistic compose snippet for image.
// source is preset | manifesto | mínimo | manual.
func BuildComposeServiceYAML(image string) (yamlText, source string) {
	image = strings.TrimSpace(image)
	manual := false
	if image == "" {
		image = "nginx:latest"
		manual = true
	}
	if p, ok := MatchComposePreset(image); ok {
		src := ComposeSourcePreset
		if manual {
			src = ComposeSourceManual
		}
		return renderComposeFromPreset(image, p), src
	}
	if cfg, err := InspectImageConfig(image); err == nil {
		ports := portsFromExposed(cfg.ExposedPorts)
		env := envMapFromSlice(cfg.Env)
		vols := volumesFromSet(cfg.Volumes, serviceNameFromImage(image))
		if len(ports) > 0 || len(env) > 0 || len(vols) > 0 {
			return renderComposeFromPreset(image, composePreset{
				ports: ports, env: env, volumes: vols,
			}), ComposeSourceManifest
		}
	}
	return renderComposeMinimal(image), ComposeSourceMinimal
}

// ComposeServiceYAMLFromPreset returns YAML only when a local preset matches (no network).
func ComposeServiceYAMLFromPreset(image string) (yamlText, source string, ok bool) {
	image = strings.TrimSpace(image)
	if image == "" {
		image = "nginx:latest"
	}
	p, matched := MatchComposePreset(image)
	if !matched {
		return "", "", false
	}
	return renderComposeFromPreset(image, p), ComposeSourcePreset, true
}

func renderComposeMinimal(image string) string {
	name := serviceNameFromImage(image)
	return fmt.Sprintf(`services:
  %s:
    image: %s
    restart: unless-stopped
`, name, image)
}

func renderComposeFromParts(image string, ports []string, env map[string]string, volumes []string, health string) string {
	return renderComposeFromPreset(image, composePreset{
		ports: ports, env: env, volumes: volumes, healthcheck: health,
	})
}

func renderComposeFromPreset(image string, p composePreset) string {
	name := serviceNameFromImage(image)
	var b strings.Builder
	b.WriteString("services:\n")
	b.WriteString("  " + name + ":\n")
	b.WriteString("    image: " + image + "\n")
	b.WriteString("    restart: unless-stopped\n")
	if wd := strings.TrimSpace(p.workingDir); wd != "" {
		b.WriteString("    working_dir: " + wd + "\n")
	}
	if len(p.ports) > 0 {
		b.WriteString("    ports:\n")
		for _, port := range p.ports {
			b.WriteString(fmt.Sprintf("      - %q\n", port))
		}
	}
	if len(p.env) > 0 {
		b.WriteString("    environment:\n")
		keys := make([]string, 0, len(p.env))
		for k := range p.env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("      %s: %q\n", k, p.env[k]))
		}
	}
	volNames := make([]string, 0, len(p.volumes))
	if len(p.volumes) > 0 {
		b.WriteString("    volumes:\n")
		for _, v := range p.volumes {
			b.WriteString(fmt.Sprintf("      - %s\n", v))
			if src, _, ok := strings.Cut(v, ":"); ok && isComposeNamedVolume(src) {
				volNames = append(volNames, src)
			}
		}
	}
	if cmd := strings.TrimSpace(p.command); cmd != "" {
		b.WriteString("    command: " + cmd + "\n")
	}
	if strings.TrimSpace(p.healthcheck) != "" {
		b.WriteString("    healthcheck:\n")
		for _, line := range strings.Split(p.healthcheck, "\n") {
			b.WriteString("      " + line + "\n")
		}
	}
	if len(volNames) > 0 {
		b.WriteString("volumes:\n")
		seen := map[string]bool{}
		for _, vn := range volNames {
			if seen[vn] {
				continue
			}
			seen[vn] = true
			b.WriteString("  " + vn + ":\n")
		}
	}
	return b.String()
}

// isComposeNamedVolume reports whether src should be declared under top-level volumes:.
func isComposeNamedVolume(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" || src == "." || src == ".." {
		return false
	}
	if strings.HasPrefix(src, ".") || strings.HasPrefix(src, "/") || strings.HasPrefix(src, "~") {
		return false
	}
	if strings.Contains(src, "/") || strings.Contains(src, "\\") {
		return false
	}
	return true
}

func composeImageBaseName(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if i := strings.Index(image, "@"); i >= 0 {
		image = image[:i]
	}
	if i := strings.Index(image, ":"); i >= 0 {
		// only strip tag if it's after the last slash (avoid registry:5000/...)
		slash := strings.LastIndex(image, "/")
		if slash < 0 || i > slash {
			image = image[:i]
		}
	}
	image = strings.TrimPrefix(image, "library/")
	if i := strings.LastIndex(image, "/"); i >= 0 {
		image = image[i+1:]
	}
	return strings.ToLower(strings.TrimSpace(image))
}

func portsFromExposed(exposed map[string]struct{}) []string {
	if len(exposed) == 0 {
		return nil
	}
	keys := make([]string, 0, len(exposed))
	for k := range exposed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		port := strings.Split(k, "/")[0]
		port = strings.TrimSpace(port)
		if port == "" {
			continue
		}
		out = append(out, port+":"+port)
	}
	return out
}

func envMapFromSlice(env []string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, e := range env {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		// Skip noisy / metadata vars from image defaults
		switch k {
		case "PATH", "LANG", "LANGUAGE", "LC_ALL", "HOME", "HOSTNAME", "TERM",
			"GPG_KEY", "OLDPWD", "PWD", "SHLVL":
			continue
		}
		if strings.HasSuffix(k, "_VERSION") || strings.HasSuffix(k, "_HOME") {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func volumesFromSet(vols map[string]struct{}, service string) []string {
	if len(vols) == 0 {
		return nil
	}
	paths := make([]string, 0, len(vols))
	for p := range vols {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]string, 0, len(paths))
	for i, p := range paths {
		name := service + "_data"
		if i > 0 {
			name = fmt.Sprintf("%s_data_%d", service, i+1)
		}
		out = append(out, name+":"+p)
	}
	return out
}
