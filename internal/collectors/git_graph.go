package collectors

import (
	"strconv"
	"strings"
)

// DAGCommit is one commit's raw parent relationships and metadata, used to
// compute the graph layout ourselves (lanes, branch/merge curves) rather
// than parsing git's own `--graph` ASCII art.
type DAGCommit struct {
	Hash        string
	Short       string
	Parents     []string // full hashes
	Author      string
	AuthorEmail string
	Date        string
	Subject     string
	Refs        []string // branch/tag names pointing here, "HEAD" included when detached
	IsHead      bool
}

const dagFieldSep = "\x1f"

// GitLogDAG returns commits across every branch (--all), topo-ordered so a
// commit never appears before any of its children — the graph layout
// algorithm depends on that ordering.
func GitLogDAG(projectPath string, limit int) []DAGCommit {
	if limit <= 0 {
		limit = 300
	}
	format := strings.Join([]string{"%H", "%h", "%P", "%an", "%ae", "%ad", "%s", "%D"}, dagFieldSep)
	out := gitOutput(projectPath, "log", "--all", "--topo-order", "--date=short",
		"--pretty=format:"+format, "-n", strconv.Itoa(limit))
	if out == "" {
		return nil
	}

	headRef := strings.TrimSpace(gitOutput(projectPath, "rev-parse", "HEAD"))

	var commits []DAGCommit
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, dagFieldSep)
		get := func(i int) string {
			if i < len(fields) {
				return fields[i]
			}
			return ""
		}
		c := DAGCommit{
			Hash:        get(0),
			Short:       get(1),
			Author:      get(3),
			AuthorEmail: get(4),
			Date:        get(5),
			Subject:     get(6),
		}
		if c.Hash == "" {
			continue
		}
		if parents := strings.Fields(get(2)); len(parents) > 0 {
			c.Parents = parents
		}
		c.IsHead = c.Hash == headRef
		if refs := strings.TrimSpace(get(7)); refs != "" {
			for _, r := range strings.Split(refs, ",") {
				r = strings.TrimSpace(r)
				r = strings.TrimPrefix(r, "HEAD -> ")
				if r == "HEAD" {
					continue
				}
				if r != "" {
					c.Refs = append(c.Refs, r)
				}
			}
		}
		commits = append(commits, c)
	}
	return commits
}

// CollectCommitFileStats is defined below alongside the DAG collector so
// both live next to the rest of the graph-screen data fetching.

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
