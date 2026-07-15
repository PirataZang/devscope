package detectors

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type baseDetector struct {
	name     string
	priority int
	language string
}

func fileExists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

func readJSONField(root, file, field string) string {
	data, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		return ""
	}
	var m map[string]interface{}
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	v, ok := m[field].(string)
	if !ok {
		return ""
	}
	return v
}

func depsContain(root, dep string) bool {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), dep)
}

// NestJS
type NestJS struct{}

func (d *NestJS) Name() string     { return "nestjs" }
func (d *NestJS) Priority() int    { return 90 }
func (d *NestJS) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "package.json") && depsContain(root, "@nestjs/core") {
		return &FrameworkInfo{Name: "NestJS", Language: "TypeScript"}, nil
	}
	return nil, nil
}

// Laravel
type Laravel struct{}

func (d *Laravel) Name() string  { return "laravel" }
func (d *Laravel) Priority() int { return 90 }
func (d *Laravel) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "artisan") && fileExists(root, "composer.json") {
		return &FrameworkInfo{Name: "Laravel", Language: "PHP"}, nil
	}
	return nil, nil
}

// Django
type Django struct{}

func (d *Django) Name() string  { return "django" }
func (d *Django) Priority() int { return 85 }
func (d *Django) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "manage.py") {
		return &FrameworkInfo{Name: "Django", Language: "Python"}, nil
	}
	return nil, nil
}

// Next.js
type NextJS struct{}

func (d *NextJS) Name() string  { return "next" }
func (d *NextJS) Priority() int { return 88 }
func (d *NextJS) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	for _, f := range []string{"next.config.js", "next.config.mjs", "next.config.ts"} {
		if fileExists(root, f) {
			return &FrameworkInfo{Name: "Next.js", Language: "TypeScript"}, nil
		}
	}
	if depsContain(root, "\"next\"") {
		return &FrameworkInfo{Name: "Next.js", Language: "TypeScript"}, nil
	}
	return nil, nil
}

// Nuxt
type NuxtJS struct{}

func (d *NuxtJS) Name() string  { return "nuxt" }
func (d *NuxtJS) Priority() int { return 88 }
func (d *NuxtJS) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	for _, f := range []string{"nuxt.config.js", "nuxt.config.ts"} {
		if fileExists(root, f) {
			return &FrameworkInfo{Name: "Nuxt", Language: "TypeScript"}, nil
		}
	}
	return nil, nil
}

// Vue
type Vue struct{}

func (d *Vue) Name() string  { return "vue" }
func (d *Vue) Priority() int { return 70 }
func (d *Vue) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "package.json") && (depsContain(root, "\"vue\"") || fileExists(root, "vite.config.ts") || fileExists(root, "vite.config.js")) {
		if depsContain(root, "\"nuxt\"") {
			return nil, nil
		}
		return &FrameworkInfo{Name: "Vue", Language: "TypeScript"}, nil
	}
	return nil, nil
}

// React
type React struct{}

func (d *React) Name() string  { return "react" }
func (d *React) Priority() int { return 65 }
func (d *React) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "package.json") && depsContain(root, "\"react\"") {
		if depsContain(root, "\"next\"") {
			return nil, nil
		}
		return &FrameworkInfo{Name: "React", Language: "TypeScript"}, nil
	}
	return nil, nil
}

// Node (generic)
type Node struct{}

func (d *Node) Name() string  { return "node" }
func (d *Node) Priority() int { return 50 }
func (d *Node) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "package.json") {
		return &FrameworkInfo{Name: "Node.js", Language: "JavaScript"}, nil
	}
	return nil, nil
}

// PHP (generic)
type PHP struct{}

func (d *PHP) Name() string  { return "php" }
func (d *PHP) Priority() int { return 45 }
func (d *PHP) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "composer.json") {
		return &FrameworkInfo{Name: "PHP", Language: "PHP"}, nil
	}
	return nil, nil
}

// Go
type GoLang struct{}

func (d *GoLang) Name() string  { return "go" }
func (d *GoLang) Priority() int { return 60 }
func (d *GoLang) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "go.mod") {
		return &FrameworkInfo{Name: "Go", Language: "Go"}, nil
	}
	return nil, nil
}

// Rust
type Rust struct{}

func (d *Rust) Name() string  { return "rust" }
func (d *Rust) Priority() int { return 60 }
func (d *Rust) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "Cargo.toml") {
		return &FrameworkInfo{Name: "Rust", Language: "Rust"}, nil
	}
	return nil, nil
}

// Python
type Python struct{}

func (d *Python) Name() string  { return "python" }
func (d *Python) Priority() int { return 55 }
func (d *Python) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "requirements.txt") || fileExists(root, "pyproject.toml") {
		return &FrameworkInfo{Name: "Python", Language: "Python"}, nil
	}
	return nil, nil
}

// Java
type Java struct{}

func (d *Java) Name() string  { return "java" }
func (d *Java) Priority() int { return 55 }
func (d *Java) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	if fileExists(root, "pom.xml") {
		return &FrameworkInfo{Name: "Maven", Language: "Java"}, nil
	}
	if fileExists(root, "build.gradle") || fileExists(root, "build.gradle.kts") {
		return &FrameworkInfo{Name: "Gradle", Language: "Java"}, nil
	}
	return nil, nil
}

// Docker-only project
type DockerOnly struct{}

func (d *DockerOnly) Name() string  { return "docker" }
func (d *DockerOnly) Priority() int { return 10 }
func (d *DockerOnly) Detect(ctx context.Context, root string) (*FrameworkInfo, error) {
	hasCompose := fileExists(root, "docker-compose.yml") || fileExists(root, "docker-compose.yaml") ||
		fileExists(root, "compose.yml") || fileExists(root, "compose.yaml")
	hasDockerfile := fileExists(root, "Dockerfile")
	if (hasCompose || hasDockerfile) && !fileExists(root, "package.json") && !fileExists(root, "go.mod") {
		return &FrameworkInfo{Name: "Docker", Language: "Container"}, nil
	}
	return nil, nil
}
