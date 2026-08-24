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
	Prefix      string
	Hash        string
	Short       string
	Author      string
	AuthorEmail string
	Date        string
	Subject     string
	Refs        string
	Parent      string // first parent, short form — "" for a root commit
}

const gitGraphFieldSep = "\x1f"

var gitGraphLineRe = regexp.MustCompile(`^(.*?)([0-9a-f]{40}` + gitGraphFieldSep + `.*)$`)

// GitLogGraph returns the combined graph of every branch (--all), newest
// first, capped at limit rows.
func GitLogGraph(projectPath string, limit int) []GitGraphRow {
	if limit <= 0 {
		limit = 300
	}
	format := strings.Join([]string{"%H", "%h", "%an", "%ae", "%ad", "%s", "%D", "%P"}, gitGraphFieldSep)
	out := gitOutput(projectPath, "log", "--all", "--date-order", "--graph", "--date=short",
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
		get := func(i int) string {
			if i < len(fields) {
				return fields[i]
			}
			return ""
		}
		row.Hash = get(0)
		row.Short = get(1)
		row.Author = get(2)
		row.AuthorEmail = get(3)
		row.Date = get(4)
		row.Subject = get(5)
		row.Refs = get(6)
		if parents := strings.Fields(get(7)); len(parents) > 0 {
			p := parents[0]
			if len(p) > 8 {
				p = p[:8]
			}
			row.Parent = p
		}
		rows = append(rows, row)
	}
	return rows
}

// GitCommitFileStat is one file's line-change count for a commit, from
// `git show --numstat`.
type GitCommitFileStat struct {
	Path       string
	Insertions int
	Deletions  int
}

// CollectCommitFileStats returns per-file +/- line counts for a commit.
func CollectCommitFileStats(projectPath, hash string) []GitCommitFileStat {
	out := gitOutput(projectPath, "show", "--numstat", "--pretty=format:", hash)
	if out == "" {
		return nil
	}
	var stats []GitCommitFileStat
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		ins, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		stats = append(stats, GitCommitFileStat{Path: parts[2], Insertions: ins, Deletions: del})
	}
	return stats
}
