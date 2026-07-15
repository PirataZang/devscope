package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// dirMarkers holds project signals found in a single directory listing.
type dirMarkers struct {
	Git           bool
	Env           bool
	PackageJSON   bool
	DockerCompose bool
	Dockerfile    bool
	GoMod         bool
	ComposerJSON  bool
	PyProject     bool
	Requirements  bool
	CargoToml     bool
	PomXML        bool
	Gradle        bool
	ManagePy      bool
	Procfile      bool
	Artisan       bool
}

func (m dirMarkers) hasFast() bool {
	return m.Git || m.Env || m.PackageJSON || m.DockerCompose || m.Dockerfile ||
		m.GoMod || m.ComposerJSON || m.PyProject || m.Requirements || m.CargoToml
}

func (m dirMarkers) hasFull() bool {
	return m.hasFast() || m.PomXML || m.Gradle || m.ManagePy || m.Procfile || m.Artisan
}

func readDirMarkers(dir string) (dirMarkers, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dirMarkers{}, false
	}

	var m dirMarkers
	for _, e := range entries {
		classifyEntry(e.Name(), e.IsDir(), &m)
	}
	return m, m.hasFull()
}

func readFastMarkers(dir string) (dirMarkers, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dirMarkers{}, false
	}

	var m dirMarkers
	for _, e := range entries {
		classifyEntry(e.Name(), e.IsDir(), &m)
	}
	return m, m.hasFast()
}

func classifyEntry(name string, isDir bool, m *dirMarkers) {
	switch name {
	case ".git":
		if isDir {
			m.Git = true
		}
	case ".env", ".env.local", ".env.example", ".env.development":
		m.Env = true
	case "package.json":
		m.PackageJSON = true
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		m.DockerCompose = true
	case "Dockerfile":
		m.Dockerfile = true
	case "go.mod":
		m.GoMod = true
	case "composer.json":
		m.ComposerJSON = true
	case "pyproject.toml":
		m.PyProject = true
	case "requirements.txt":
		m.Requirements = true
	case "Cargo.toml":
		m.CargoToml = true
	case "pom.xml":
		m.PomXML = true
	case "build.gradle", "build.gradle.kts":
		m.Gradle = true
	case "manage.py":
		m.ManagePy = true
	case "Procfile":
		m.Procfile = true
	case "artisan":
		m.Artisan = true
	}
}

func hasProjectMarker(dir string) bool {
	_, ok := readDirMarkers(dir)
	return ok
}

var roleDirs = map[string]string{
	"frontend": "frontend",
	"front":    "frontend",
	"client":   "frontend",
	"web":      "frontend",
	"ui":       "frontend",
	"backend":  "backend",
	"api":      "backend",
	"server":   "backend",
	"worker":   "worker",
	"workers":  "worker",
	"cron":     "cron",
	"jobs":     "worker",
	"redis":    "redis",
	"postgres": "postgres",
	"database": "postgres",
	"db":       "postgres",
	"mongo":    "mongo",
}

func isIgnored(name string, ignore []string) bool {
	for _, ig := range ignore {
		if name == ig {
			return true
		}
	}
	return false
}

func detectModules(root string) []moduleInfo {
	var modules []moduleInfo
	entries, err := os.ReadDir(root)
	if err != nil {
		return modules
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		role, ok := roleDirs[strings.ToLower(e.Name())]
		if !ok {
			continue
		}
		modules = append(modules, moduleInfo{
			Name: e.Name(),
			Path: filepath.Join(root, e.Name()),
			Role: role,
		})
	}
	return modules
}

type moduleInfo struct {
	Name string
	Path string
	Role string
}

func hasFile(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

func (m dirMarkers) hasCode() bool {
	return m.Git || m.Env || m.PackageJSON || m.GoMod || m.ComposerJSON ||
		m.PyProject || m.Requirements || m.CargoToml || m.PomXML ||
		m.Gradle || m.ManagePy || m.Artisan
}

func isServiceSubfolder(path string, markers dirMarkers) bool {
	if markers.hasCode() {
		return false
	}
	if !markers.Dockerfile && !markers.DockerCompose {
		return false
	}
	parent := filepath.Dir(path)
	parentMarkers, ok := readFastMarkers(parent)
	if !ok {
		return false
	}
	return parentMarkers.hasFast()
}

func guessFromMarkers(m dirMarkers) (framework, language string) {
	switch {
	case m.GoMod:
		return "Go", "Go"
	case m.ComposerJSON || m.Artisan:
		return "PHP", "PHP"
	case m.PackageJSON:
		return "Node", "JavaScript"
	case m.PyProject || m.Requirements || m.ManagePy:
		return "Python", "Python"
	case m.CargoToml:
		return "Rust", "Rust"
	case m.PomXML || m.Gradle:
		return "Java", "Java"
	case m.DockerCompose || m.Dockerfile:
		return "Docker", "Docker"
	case m.Git:
		return "Git", "Unknown"
	case m.Env:
		return "App", "Unknown"
	default:
		return "Unknown", "Unknown"
	}
}
