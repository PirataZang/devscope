package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/core"
)

type DBEngine string

const (
	DBEnginePostgres DBEngine = "postgres"
	DBEngineMySQL    DBEngine = "mysql"
)

// DBTarget is a runnable database reachable via docker exec.
type DBTarget struct {
	Label     string
	Container string
	Engine    DBEngine
	User      string
	Database  string
}

var (
	envDatabaseURLRe = regexp.MustCompile(`(?m)^(?:DATABASE_URL|DB_URL|POSTGRES_URL|MYSQL_URL)\s*=\s*["']?([^\s"']+)`)
	envDBUserRe      = regexp.MustCompile(`(?m)^(?:POSTGRES_USER|MYSQL_USER|DB_USER|DB_USERNAME)\s*=\s*["']?([^\s"']+)`)
	envDBNameRe      = regexp.MustCompile(`(?m)^(?:POSTGRES_DB|MYSQL_DATABASE|DB_DATABASE|DB_NAME)\s*=\s*["']?([^\s"']+)`)
	envDBPassRe      = regexp.MustCompile(`(?m)^(?:POSTGRES_PASSWORD|MYSQL_PASSWORD|DB_PASSWORD)\s*=\s*["']?([^\s"']+)`)
)

// DetectProjectDatabases finds Postgres/MySQL containers linked to the project.
func DetectProjectDatabases(p *core.Project) []DBTarget {
	if p == nil {
		return nil
	}
	envUser, envDB, _ := readProjectDBEnv(p.Path)
	var out []DBTarget
	seen := map[string]bool{}
	for _, c := range p.Containers {
		eng := detectContainerDBEngine(c)
		if eng == "" {
			continue
		}
		key := DockerExecTarget(c)
		if seen[key] {
			continue
		}
		seen[key] = true
		user, db := envUser, envDB
		if user == "" || db == "" {
			cu, cd := readContainerDBEnv(key, eng)
			if user == "" {
				user = cu
			}
			if db == "" {
				db = cd
			}
		}
		if user == "" {
			if eng == DBEnginePostgres {
				user = "postgres"
			} else {
				user = "root"
			}
		}
		if db == "" {
			if eng == DBEnginePostgres {
				db = "postgres"
			} else {
				db = user
			}
		}
		out = append(out, DBTarget{
			Label:     c.Name,
			Container: key,
			Engine:    eng,
			User:      user,
			Database:  db,
		})
	}
	return out
}

func detectContainerDBEngine(c core.Container) DBEngine {
	s := strings.ToLower(c.Image + " " + c.Name)
	switch {
	case strings.Contains(s, "postgres"), strings.Contains(s, "postgis"):
		return DBEnginePostgres
	case strings.Contains(s, "mysql"), strings.Contains(s, "mariadb"), strings.Contains(s, "percona"):
		return DBEngineMySQL
	default:
		return ""
	}
}

func readProjectDBEnv(projectPath string) (user, db, pass string) {
	for _, name := range []string{".env", ".env.local", ".env.production", ".env.example"} {
		data, err := os.ReadFile(filepath.Join(projectPath, name))
		if err != nil {
			continue
		}
		if m := envDatabaseURLRe.FindSubmatch(data); len(m) > 1 {
			u, d, p := parseDatabaseURL(string(m[1]))
			if user == "" {
				user = u
			}
			if db == "" {
				db = d
			}
			if pass == "" {
				pass = p
			}
		}
		if user == "" {
			if m := envDBUserRe.FindSubmatch(data); len(m) > 1 {
				user = string(m[1])
			}
		}
		if db == "" {
			if m := envDBNameRe.FindSubmatch(data); len(m) > 1 {
				db = string(m[1])
			}
		}
		if pass == "" {
			if m := envDBPassRe.FindSubmatch(data); len(m) > 1 {
				pass = string(m[1])
			}
		}
	}
	return user, db, pass
}

func parseDatabaseURL(raw string) (user, db, pass string) {
	// postgres://user:pass@host:5432/dbname
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if at := strings.Index(raw, "@"); at >= 0 {
		cred := raw[:at]
		if colon := strings.Index(cred, ":"); colon >= 0 {
			user = cred[:colon]
			pass = cred[colon+1:]
		} else {
			user = cred
		}
		raw = raw[at+1:]
	}
	if slash := strings.Index(raw, "/"); slash >= 0 {
		db = strings.TrimSuffix(raw[slash+1:], "/")
		if q := strings.Index(db, "?"); q >= 0 {
			db = db[:q]
		}
	}
	return user, db, pass
}

func readContainerDBEnv(container string, eng DBEngine) (user, db string) {
	keys := []string{"POSTGRES_USER", "POSTGRES_DB", "MYSQL_USER", "MYSQL_DATABASE", "MYSQL_USER", "MARIADB_USER", "MARIADB_DATABASE"}
	env := map[string]string{}
	for _, k := range keys {
		out, err := exec.Command("docker", "exec", container, "printenv", k).CombinedOutput()
		if err != nil {
			continue
		}
		env[k] = strings.TrimSpace(string(out))
	}
	switch eng {
	case DBEnginePostgres:
		user = firstNonEmpty(env["POSTGRES_USER"], "postgres")
		db = firstNonEmpty(env["POSTGRES_DB"], user, "postgres")
	case DBEngineMySQL:
		user = firstNonEmpty(env["MYSQL_USER"], env["MARIADB_USER"], "root")
		db = firstNonEmpty(env["MYSQL_DATABASE"], env["MARIADB_DATABASE"], user)
	}
	return user, db
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func containerPass(container string, eng DBEngine, projectPath string) string {
	_, _, pass := readProjectDBEnv(projectPath)
	if pass != "" {
		return pass
	}
	key := "POSTGRES_PASSWORD"
	if eng == DBEngineMySQL {
		key = "MYSQL_PASSWORD"
	}
	out, err := exec.Command("docker", "exec", container, "printenv", key).CombinedOutput()
	if err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return p
		}
	}
	if eng == DBEngineMySQL {
		out, err = exec.Command("docker", "exec", container, "printenv", "MYSQL_ROOT_PASSWORD").CombinedOutput()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// DBListTables returns table names for the target.
func DBListTables(t DBTarget, projectPath string) ([]string, error) {
	pass := containerPass(t.Container, t.Engine, projectPath)
	var cmd *exec.Cmd
	switch t.Engine {
	case DBEnginePostgres:
		sql := `SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY 1`
		args := []string{"exec"}
		if pass != "" {
			args = append(args, "-e", "PGPASSWORD="+pass)
		}
		args = append(args, t.Container, "psql", "-U", t.User, "-d", t.Database, "-Atc", sql)
		cmd = exec.Command("docker", args...)
	case DBEngineMySQL:
		sql := "SHOW TABLES"
		args := []string{"exec"}
		if pass != "" {
			args = append(args, "-e", "MYSQL_PWD="+pass)
		}
		args = append(args, t.Container, "mysql", "-u"+t.User, "-N", "-e", sql, t.Database)
		cmd = exec.Command("docker", args...)
	default:
		return nil, fmt.Errorf("engine não suportado")
	}
	out, err := runDBCmd(cmd)
	if err != nil {
		return nil, err
	}
	var tables []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tables = append(tables, line)
		}
	}
	return tables, nil
}

// DBQuery runs SQL and returns tabular text (truncated).
func DBQuery(t DBTarget, projectPath, sql string) (string, error) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return "", fmt.Errorf("SQL vazio")
	}
	pass := containerPass(t.Container, t.Engine, projectPath)
	var cmd *exec.Cmd
	switch t.Engine {
	case DBEnginePostgres:
		args := []string{"exec"}
		if pass != "" {
			args = append(args, "-e", "PGPASSWORD="+pass)
		}
		args = append(args, t.Container, "psql", "-U", t.User, "-d", t.Database, "-c", sql)
		cmd = exec.Command("docker", args...)
	case DBEngineMySQL:
		args := []string{"exec"}
		if pass != "" {
			args = append(args, "-e", "MYSQL_PWD="+pass)
		}
		args = append(args, t.Container, "mysql", "-u"+t.User, "-t", "-e", sql, t.Database)
		cmd = exec.Command("docker", args...)
	default:
		return "", fmt.Errorf("engine não suportado")
	}
	out, err := runDBCmd(cmd)
	if err != nil {
		return out, err
	}
	return truncateDBOutput(out, 80_000), nil
}

func runDBCmd(cmd *exec.Cmd) (string, error) {
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
		s := strings.TrimSpace(string(out))
		if err != nil {
			if s != "" {
				return s, fmt.Errorf("%s", s)
			}
			return "", err
		}
		return s, nil
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("timeout (20s)")
	}
}

func truncateDBOutput(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "\n… (truncado)"
}
