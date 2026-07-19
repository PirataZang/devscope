package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

const dbResultBodyLimit = 2 * 1024 * 1024

type dbEngine int

const (
	dbEnginePostgres dbEngine = iota
	dbEngineMySQL
	dbEngineRedis
	dbEngineMongo
)

func (e dbEngine) String() string {
	switch e {
	case dbEnginePostgres:
		return "Postgres"
	case dbEngineMySQL:
		return "MySQL"
	case dbEngineRedis:
		return "Redis"
	case dbEngineMongo:
		return "Mongo"
	default:
		return "Postgres"
	}
}

func (e dbEngine) CLI() string {
	switch e {
	case dbEnginePostgres:
		return "psql"
	case dbEngineMySQL:
		return "mysql"
	case dbEngineRedis:
		return "redis-cli"
	case dbEngineMongo:
		return "mongosh"
	default:
		return "psql"
	}
}

func (e dbEngine) DefaultPort() int {
	switch e {
	case dbEnginePostgres:
		return 5432
	case dbEngineMySQL:
		return 3306
	case dbEngineRedis:
		return 6379
	case dbEngineMongo:
		return 27017
	default:
		return 5432
	}
}

func (e dbEngine) DefaultUser() string {
	switch e {
	case dbEnginePostgres:
		return "postgres"
	case dbEngineMySQL:
		return "root"
	case dbEngineMongo:
		return ""
	default:
		return ""
	}
}

func (e dbEngine) DefaultDatabase() string {
	switch e {
	case dbEnginePostgres:
		return "postgres"
	case dbEngineMySQL:
		return "mysql"
	case dbEngineRedis:
		return "0"
	case dbEngineMongo:
		return "test"
	default:
		return ""
	}
}

func (e dbEngine) DefaultQuery() string {
	switch e {
	case dbEnginePostgres:
		return "SELECT version();"
	case dbEngineMySQL:
		return "SELECT VERSION();"
	case dbEngineRedis:
		return "PING"
	case dbEngineMongo:
		return "db.runCommand({ ping: 1 })"
	default:
		return ""
	}
}

var dbEngines = []dbEngine{dbEnginePostgres, dbEngineMySQL, dbEngineRedis, dbEngineMongo}

type dbTarget struct {
	Label     string
	Engine    dbEngine
	Host      string
	Port      int
	User      string
	Password  string
	Database  string
	Container string // docker container name/id; empty = host CLI
	Source    string // container image / service hint
}

type dbRequest struct {
	Engine    dbEngine
	Host      string
	Port      int
	User      string
	Password  string
	Database  string
	Container string
	Query     string
}

type dbResultMsg struct {
	body     string
	err      error
	duration time.Duration
	engine   string
	rowsHint string
}

type dbColumnInfo struct {
	Name     string
	Type     string
	Nullable string
}

type dbSchemaMsg struct {
	tables []string
	err    error
	engine string
}

type dbColumnsMsg struct {
	table   string
	columns []dbColumnInfo
	err     error
}

func sendDBQuery(req dbRequest) tea.Cmd {
	return func() tea.Msg {
		q := strings.TrimSpace(req.Query)
		if q == "" {
			return dbResultMsg{err: fmt.Errorf("query vazia"), engine: req.Engine.String()}
		}
		start := time.Now()
		body, err := execDBQuery(req)
		duration := time.Since(start)
		if err != nil {
			return dbResultMsg{err: err, duration: duration, engine: req.Engine.String()}
		}
		body = truncateAPIBody(body, dbResultBodyLimit)
		return dbResultMsg{
			body:     body,
			duration: duration,
			engine:   req.Engine.String(),
			rowsHint: dbRowsHint(body),
		}
	}
}

func execDBQuery(req dbRequest) (string, error) {
	if req.Container != "" {
		return execDBViaDocker(req)
	}
	return execDBViaHost(req)
}

func execDBViaHost(req dbRequest) (string, error) {
	cli := req.Engine.CLI()
	path, err := exec.LookPath(cli)
	if err != nil {
		return "", fmt.Errorf("%s não encontrado no PATH\n\ndica: instale o client ou use um container do projeto (docker exec)", cli)
	}
	args, env, err := dbHostArgs(req)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(path, args...)
	cmd.Env = append(os.Environ(), env...)
	return runDBCommand(cmd)
}

func execDBViaDocker(req dbRequest) (string, error) {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return "", fmt.Errorf("docker não encontrado no PATH")
	}
	container := strings.TrimSpace(req.Container)
	if container == "" {
		return "", fmt.Errorf("container docker vazio")
	}
	inner, envPass, err := dbDockerInner(req)
	if err != nil {
		return "", err
	}
	// Do not use -i: under a bubbletea TUI on Windows, inheriting the
	// console stdin makes CreateProcess fail with "invalid argument".
	args := []string{"exec"}
	if envPass != "" {
		args = append(args, "-e", envPass)
	}
	args = append(args, container)
	args = append(args, inner...)
	cmd := exec.Command(dockerPath, args...)
	return runDBCommand(cmd)
}

// runDBCommand executes a DB CLI/docker command without attaching the TUI stdin.
func runDBCommand(cmd *exec.Cmd) (string, error) {
	// Empty stdin avoids Windows console-handle inheritance issues while the TUI is in raw mode.
	cmd.Stdin = bytes.NewReader(nil)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Platform-specific: detach from TUI console on Windows (see db_exec_*.go).
	configureDBCommand(cmd)
	runErr := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errText := strings.TrimSpace(stderr.String())
	if runErr != nil {
		if errText != "" {
			return "", fmt.Errorf("%s", errText)
		}
		// Surface the command so "invalid argument" is diagnosable.
		return "", fmt.Errorf("%s\ncmd: %s %s", runErr.Error(), cmd.Path, strings.Join(cmd.Args[1:], " "))
	}
	if out == "" && errText != "" {
		return errText, nil
	}
	return out, nil
}

func dbHostArgs(req dbRequest) (args []string, env []string, err error) {
	host := strings.TrimSpace(req.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := req.Port
	if port <= 0 {
		port = req.Engine.DefaultPort()
	}
	user := strings.TrimSpace(req.User)
	db := strings.TrimSpace(req.Database)
	pass := req.Password

	switch req.Engine {
	case dbEnginePostgres:
		if user == "" {
			user = "postgres"
		}
		if db == "" {
			db = "postgres"
		}
		args = []string{
			"-h", host,
			"-p", strconv.Itoa(port),
			"-U", user,
			"-d", db,
			"-v", "ON_ERROR_STOP=1",
			"-P", "pager=off",
			"-c", req.Query,
		}
		if pass != "" {
			env = append(env, "PGPASSWORD="+pass)
		}
		return args, env, nil
	case dbEngineMySQL:
		if user == "" {
			user = "root"
		}
		args = []string{
			"-h", host,
			"-P", strconv.Itoa(port),
			"-u", user,
			"--batch",
			"--raw",
			"-e", req.Query,
		}
		if db != "" {
			args = append(args, db)
		}
		if pass != "" {
			env = append(env, "MYSQL_PWD="+pass)
		}
		return args, env, nil
	case dbEngineRedis:
		args = []string{"-h", host, "-p", strconv.Itoa(port)}
		if pass != "" {
			args = append(args, "-a", pass, "--no-auth-warning")
		}
		if db != "" && db != "0" {
			args = append(args, "-n", db)
		}
		args = append(args, strings.Fields(req.Query)...)
		return args, env, nil
	case dbEngineMongo:
		uri := fmt.Sprintf("mongodb://%s:%d", host, port)
		if user != "" {
			if pass != "" {
				uri = fmt.Sprintf("mongodb://%s:%s@%s:%d", user, pass, host, port)
			} else {
				uri = fmt.Sprintf("mongodb://%s@%s:%d", user, host, port)
			}
		}
		if db == "" {
			db = "test"
		}
		args = []string{uri + "/" + db, "--quiet", "--eval", req.Query}
		return args, env, nil
	default:
		return nil, nil, fmt.Errorf("engine desconhecido")
	}
}

func dbDockerInner(req dbRequest) (args []string, envPass string, err error) {
	user := strings.TrimSpace(req.User)
	db := strings.TrimSpace(req.Database)
	pass := req.Password

	switch req.Engine {
	case dbEnginePostgres:
		if user == "" {
			user = "postgres"
		}
		if db == "" {
			db = "postgres"
		}
		args = []string{
			"psql",
			"-U", user,
			"-d", db,
			"-v", "ON_ERROR_STOP=1",
			"-P", "pager=off",
			"-c", req.Query,
		}
		if pass != "" {
			envPass = "PGPASSWORD=" + pass
		}
		return args, envPass, nil
	case dbEngineMySQL:
		if user == "" {
			user = "root"
		}
		args = []string{"mysql", "-u", user, "--batch", "--raw", "-e", req.Query}
		if db != "" {
			args = append(args, db)
		}
		if pass != "" {
			envPass = "MYSQL_PWD=" + pass
		}
		return args, envPass, nil
	case dbEngineRedis:
		args = []string{"redis-cli"}
		if pass != "" {
			args = append(args, "-a", pass, "--no-auth-warning")
		}
		if db != "" && db != "0" {
			args = append(args, "-n", db)
		}
		args = append(args, strings.Fields(req.Query)...)
		return args, "", nil
	case dbEngineMongo:
		if db == "" {
			db = "test"
		}
		// mongosh inside container; prefer local eval
		args = []string{"mongosh", db, "--quiet", "--eval", req.Query}
		return args, "", nil
	default:
		return nil, "", fmt.Errorf("engine desconhecido")
	}
}

func dbRowsHint(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	n := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d linhas", n)
}

func sendDBSchema(req dbRequest) tea.Cmd {
	return func() tea.Msg {
		q := schemaTablesQuery(req.Engine)
		if q == "" {
			return dbSchemaMsg{engine: req.Engine.String()}
		}
		req.Query = q
		body, err := execDBQuery(req)
		if err != nil {
			return dbSchemaMsg{err: err, engine: req.Engine.String()}
		}
		return dbSchemaMsg{
			tables: parseSchemaTableList(req.Engine, body),
			engine: req.Engine.String(),
		}
	}
}

func sendDBColumns(req dbRequest, table string) tea.Cmd {
	return func() tea.Msg {
		table = strings.TrimSpace(table)
		if table == "" {
			return dbColumnsMsg{table: table}
		}
		q := schemaColumnsQuery(req.Engine, table)
		if q == "" {
			return dbColumnsMsg{table: table}
		}
		req.Query = q
		body, err := execDBQuery(req)
		if err != nil {
			return dbColumnsMsg{table: table, err: err}
		}
		return dbColumnsMsg{
			table:   table,
			columns: parseSchemaColumns(req.Engine, body),
		}
	}
}

func schemaTablesQuery(engine dbEngine) string {
	switch engine {
	case dbEnginePostgres:
		return "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name;"
	case dbEngineMySQL:
		return "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name;"
	case dbEngineMongo:
		return "db.getCollectionNames().join('\\n')"
	case dbEngineRedis:
		return "KEYS *"
	default:
		return ""
	}
}

func schemaColumnsQuery(engine dbEngine, table string) string {
	safe := strings.ReplaceAll(table, "'", "''")
	switch engine {
	case dbEnginePostgres:
		return fmt.Sprintf(
			"SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '%s' ORDER BY ordinal_position;",
			safe,
		)
	case dbEngineMySQL:
		return fmt.Sprintf(
			"SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = '%s' ORDER BY ordinal_position;",
			safe,
		)
	case dbEngineMongo:
		return fmt.Sprintf("Object.keys(db.getCollection('%s').findOne() || {}).join('\\n')", strings.ReplaceAll(table, "'", "\\'"))
	case dbEngineRedis:
		return "TYPE " + table
	default:
		return ""
	}
}

func parseSchemaTableList(engine dbEngine, body string) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for i, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// skip psql/mysql headers and separators
		if engine == dbEnginePostgres || engine == dbEngineMySQL {
			if i == 0 && (strings.EqualFold(line, "table_name") || strings.EqualFold(line, "Tables_in_"+line)) {
				continue
			}
			if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") {
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "(") && strings.Contains(strings.ToLower(line), "row") {
				continue
			}
			// mysql batch: first line is header "table_name"
			if strings.EqualFold(line, "table_name") {
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "tables_in_") {
				continue
			}
		}
		if engine == dbEngineRedis && (line == "(empty list or set)" || strings.HasPrefix(line, "ERR")) {
			continue
		}
		// strip psql alignment padding
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if engine == dbEngineMySQL && strings.Contains(line, "\t") {
			name = strings.Split(line, "\t")[0]
		}
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) >= 200 {
			break
		}
	}
	return out
}

func parseSchemaColumns(engine dbEngine, body string) []dbColumnInfo {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	var out []dbColumnInfo
	for i, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "(") && strings.Contains(strings.ToLower(line), "row") {
			continue
		}
		// header row
		low := strings.ToLower(line)
		if i == 0 && (strings.Contains(low, "column_name") || strings.Contains(low, "field")) {
			continue
		}
		if engine == dbEngineRedis {
			out = append(out, dbColumnInfo{Name: "type", Type: line})
			continue
		}
		if engine == dbEngineMongo {
			out = append(out, dbColumnInfo{Name: line, Type: "field"})
			continue
		}
		var parts []string
		if strings.Contains(line, "\t") {
			parts = strings.Split(line, "\t")
		} else {
			parts = strings.Fields(line)
		}
		if len(parts) == 0 {
			continue
		}
		col := dbColumnInfo{Name: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			col.Type = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			col.Nullable = strings.TrimSpace(parts[2])
		}
		if col.Name == "" {
			continue
		}
		out = append(out, col)
		if len(out) >= 200 {
			break
		}
	}
	return out
}

func dbSuggestFromContainerEnv(container string) (user, pass, database, host string, port int) {
	container = strings.TrimSpace(container)
	if container == "" {
		return "", "", "", "", 0
	}
	raw, err := collectors.DockerContainerEnv(container)
	if err != nil || raw == "" || strings.HasPrefix(raw, "(sem") {
		return "", "", "", "", 0
	}
	vals := parseEnvKV(raw)
	user = firstNonEmpty(
		vals["POSTGRES_USER"], vals["POSTGRES_USERNAME"],
		vals["MYSQL_USER"], vals["MYSQL_ROOT_USER"],
		vals["MONGO_INITDB_ROOT_USERNAME"], vals["MONGO_USERNAME"],
		vals["DB_USER"], vals["DATABASE_USER"],
	)
	pass = firstNonEmpty(
		vals["POSTGRES_PASSWORD"],
		vals["MYSQL_PASSWORD"], vals["MYSQL_ROOT_PASSWORD"],
		vals["MONGO_INITDB_ROOT_PASSWORD"], vals["MONGO_PASSWORD"],
		vals["REDIS_PASSWORD"], vals["REDISCLI_AUTH"],
		vals["DB_PASSWORD"], vals["DATABASE_PASSWORD"],
	)
	database = firstNonEmpty(
		vals["POSTGRES_DB"], vals["POSTGRES_DATABASE"],
		vals["MYSQL_DATABASE"], vals["MYSQL_DB"],
		vals["MONGO_INITDB_DATABASE"], vals["MONGO_DATABASE"],
		vals["DB_NAME"], vals["DATABASE_NAME"],
	)
	host = firstNonEmpty(vals["DB_HOST"], vals["DATABASE_HOST"], vals["POSTGRES_HOST"], vals["MYSQL_HOST"])
	if p := firstNonEmpty(vals["DB_PORT"], vals["DATABASE_PORT"], vals["POSTGRES_PORT"], vals["MYSQL_PORT"]); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	if url := firstNonEmpty(vals["DATABASE_URL"], vals["DATABASE_URI"], vals["POSTGRES_URL"], vals["MYSQL_URL"]); url != "" {
		u, p, d, h, pt := parseDatabaseURL(url)
		user = firstNonEmpty(u, user)
		pass = firstNonEmpty(p, pass)
		database = firstNonEmpty(d, database)
		host = firstNonEmpty(h, host)
		if pt > 0 {
			port = pt
		}
	}
	return user, pass, database, host, port
}

func detectDBEngineFromImage(image, name string) (dbEngine, bool) {
	s := strings.ToLower(image + " " + name)
	switch {
	case strings.Contains(s, "postgres"), strings.Contains(s, "postgis"), strings.Contains(s, "pgvector"):
		return dbEnginePostgres, true
	case strings.Contains(s, "mysql"), strings.Contains(s, "mariadb"), strings.Contains(s, "percona"):
		return dbEngineMySQL, true
	case strings.Contains(s, "redis"), strings.Contains(s, "keydb"), strings.Contains(s, "valkey"):
		return dbEngineRedis, true
	case strings.Contains(s, "mongo"):
		return dbEngineMongo, true
	default:
		return 0, false
	}
}

func discoverDBTargets(p *core.Project) []dbTarget {
	var out []dbTarget
	seen := map[string]bool{}

	add := func(t dbTarget) {
		key := fmt.Sprintf("%s|%s|%s|%d|%s", t.Container, t.Engine.String(), t.Host, t.Port, t.Database)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, t)
	}

	if p != nil {
		for _, c := range p.Containers {
			engine, ok := detectDBEngineFromImage(c.Image, c.Name)
			if !ok {
				continue
			}
			state := strings.ToLower(c.State)
			if state != "" && state != "running" {
				continue
			}
			port := parseFirstHostPort(c.Ports, engine.DefaultPort())
			label := c.Name
			if label == "" {
				label = truncate(c.ID, 12)
			}
			add(dbTarget{
				Label:     label,
				Engine:    engine,
				Host:      "127.0.0.1",
				Port:      port,
				User:      engine.DefaultUser(),
				Database:  engine.DefaultDatabase(),
				Container: firstNonEmpty(c.Name, c.ID),
				Source:    c.Image,
			})
		}
	}

	// Host CLI fallbacks (always available as option).
	for _, e := range dbEngines {
		if _, err := exec.LookPath(e.CLI()); err != nil {
			continue
		}
		add(dbTarget{
			Label:    e.String() + " (host)",
			Engine:   e,
			Host:     "127.0.0.1",
			Port:     e.DefaultPort(),
			User:     e.DefaultUser(),
			Database: e.DefaultDatabase(),
			Source:   e.CLI(),
		})
	}

	if len(out) == 0 {
		// Still offer engines so the user can configure manually.
		for _, e := range dbEngines {
			add(dbTarget{
				Label:    e.String(),
				Engine:   e,
				Host:     "127.0.0.1",
				Port:     e.DefaultPort(),
				User:     e.DefaultUser(),
				Database: e.DefaultDatabase(),
				Source:   "manual",
			})
		}
	}
	return out
}

func parseFirstHostPort(ports string, fallback int) int {
	// docker ports like "0.0.0.0:5432->5432/tcp, :::5432->5432/tcp"
	for _, part := range strings.Split(ports, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// take left side of ->
		left := part
		if i := strings.Index(part, "->"); i >= 0 {
			left = part[:i]
		}
		// last :port
		if i := strings.LastIndex(left, ":"); i >= 0 {
			raw := left[i+1:]
			raw = strings.TrimSpace(raw)
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				return n
			}
		}
	}
	return fallback
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func dbSuggestFromEnvFiles(projectPath string) (user, pass, database, host string, port int) {
	if projectPath == "" {
		return "", "", "", "", 0
	}
	candidates := []string{
		filepath.Join(projectPath, ".env"),
		filepath.Join(projectPath, ".env.local"),
		filepath.Join(projectPath, "backend", ".env"),
		filepath.Join(projectPath, "api", ".env"),
	}
	for _, f := range candidates {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		vals := parseEnvKV(string(data))
		// common keys
		user = firstNonEmpty(vals["POSTGRES_USER"], vals["POSTGRES_USERNAME"], vals["DB_USER"], vals["DATABASE_USER"], vals["MYSQL_USER"], user)
		pass = firstNonEmpty(vals["POSTGRES_PASSWORD"], vals["DB_PASSWORD"], vals["DATABASE_PASSWORD"], vals["MYSQL_PASSWORD"], vals["MYSQL_ROOT_PASSWORD"], pass)
		database = firstNonEmpty(vals["POSTGRES_DB"], vals["DB_NAME"], vals["DATABASE_NAME"], vals["MYSQL_DATABASE"], database)
		host = firstNonEmpty(vals["DB_HOST"], vals["DATABASE_HOST"], vals["POSTGRES_HOST"], host)
		if p := firstNonEmpty(vals["DB_PORT"], vals["DATABASE_PORT"], vals["POSTGRES_PORT"]); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				port = n
			}
		}
		// DATABASE_URL / DATABASE_URI
		if url := firstNonEmpty(vals["DATABASE_URL"], vals["DATABASE_URI"], vals["POSTGRES_URL"]); url != "" {
			u, p, d, h, pt := parseDatabaseURL(url)
			user = firstNonEmpty(u, user)
			pass = firstNonEmpty(p, pass)
			database = firstNonEmpty(d, database)
			host = firstNonEmpty(h, host)
			if pt > 0 {
				port = pt
			}
		}
	}
	return user, pass, database, host, port
}

func parseEnvKV(text string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(line[7:])
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if key != "" {
			out[key] = val
		}
	}
	return out
}

// parseDatabaseURL handles postgres://user:pass@host:port/db and mysql://...
func parseDatabaseURL(raw string) (user, pass, database, host string, port int) {
	raw = strings.TrimSpace(raw)
	// strip scheme
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	// user:pass@host
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		cred := raw[:at]
		raw = raw[at+1:]
		if colon := strings.Index(cred, ":"); colon >= 0 {
			user = cred[:colon]
			pass = cred[colon+1:]
		} else {
			user = cred
		}
	}
	// host:port/db?query
	path := ""
	if slash := strings.Index(raw, "/"); slash >= 0 {
		path = raw[slash+1:]
		raw = raw[:slash]
	}
	if q := strings.Index(path, "?"); q >= 0 {
		path = path[:q]
	}
	database = path
	if colon := strings.LastIndex(raw, ":"); colon >= 0 {
		host = raw[:colon]
		if n, err := strconv.Atoi(raw[colon+1:]); err == nil {
			port = n
		}
	} else {
		host = raw
	}
	return user, pass, database, host, port
}
