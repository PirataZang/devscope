package collectors

import (
	"regexp"
	"strconv"
	"strings"
)

// GitGraphRow is one line of `git log --all --graph` output. Prefix keeps
// git's own ASCII lane layout (the hard graph-layout problem is already
// solved there); rows with no commit are pure connector lines between
// merges/branches and have an empty Hash.
type GitGraphRow struct {
	Prefix  string
	Hash    string
	Short   string
	Author  string
	When    string
	Subject string
	Refs    string
}

const gitGraphFieldSep = "\x1f"

var gitGraphLineRe = regexp.MustCompile(`^(.*?)([0-9a-f]{40}` + gitGraphFieldSep + `.*)$`)

// GitLogGraph returns the combined graph of every branch (--all), newest
// first, capped at limit rows.
func GitLogGraph(projectPath string, limit int) []GitGraphRow {
	if limit <= 0 {
		limit = 300
	}
	format := strings.Join([]string{"%H", "%h", "%an", "%ar", "%s", "%D"}, gitGraphFieldSep)
	out := gitOutput(projectPath, "log", "--all", "--date-order", "--graph",
		"--pretty=format:"+format, "-n", strconv.Itoa(limit))
	if out == "" {
		return nil
	}

	var rows []GitGraphRow
	for _, line := range strings.Split(out, "\n") {
		m := gitGraphLineRe.FindStringSubmatch(line)
		if m == nil {
			rows = append(rows, GitGraphRow{Prefix: line})
			continue
		}
		fields := strings.Split(m[2], gitGraphFieldSep)
		row := GitGraphRow{Prefix: m[1]}
		if len(fields) > 0 {
			row.Hash = fields[0]
		}
		if len(fields) > 1 {
			row.Short = fields[1]
		}
		if len(fields) > 2 {
			row.Author = fields[2]
		}
		if len(fields) > 3 {
			row.When = fields[3]
		}
		if len(fields) > 4 {
			row.Subject = fields[4]
		}
		if len(fields) > 5 {
			row.Refs = fields[5]
		}
		rows = append(rows, row)
	}
	return rows
}

// CommitsReachableFrom returns the set of commit hashes reachable from ref —
// used to highlight which graph rows belong to a branch selected in the UI.
func CommitsReachableFrom(projectPath, ref string) map[string]bool {
	out := gitOutput(projectPath, "log", ref, "--pretty=format:%H")
	set := make(map[string]bool)
	if out == "" {
		return set
	}
	for _, h := range strings.Split(out, "\n") {
		h = strings.TrimSpace(h)
		if h != "" {
			set[h] = true
		}
	}
	return set
}
