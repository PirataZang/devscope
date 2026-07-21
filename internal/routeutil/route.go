package routeutil

// Route is a discovered HTTP endpoint.
type Route struct {
	Method  string // GET, POST, PUT, PATCH, DELETE
	Path    string // /api/users/{id}
	Source  string // openapi | laravel | nestjs | django | ...
	File    string // relative path (optional)
	Line    int
	Summary string
	Auth    bool // protected / requires auth (best-effort from middleware/guards)
}

// Scanner extracts routes from a project tree for a given stack.
type Scanner interface {
	Name() string
	Match(projectPath string, stacks []string) bool
	Scan(projectPath string) ([]Route, error)
}

var scanners []Scanner

func Register(s Scanner) {
	scanners = append(scanners, s)
}

func init() {
	Register(laravelScanner{})
	Register(nestjsScanner{})
	Register(expressScanner{})
	Register(fastapiScanner{})
	Register(djangoScanner{})
	Register(flaskScanner{})
	Register(nextjsScanner{})
	Register(nuxtScanner{})
	Register(goScanner{})
	Register(springScanner{})
	Register(railsScanner{})
	Register(nodeExtraScanner{})
	Register(rustScanner{})
}
