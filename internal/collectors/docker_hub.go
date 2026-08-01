package collectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type DockerHubRepo struct {
	Name        string
	Description string
	Stars       int
	Pulls       int64
	Official    bool
	Automated   bool
}

type DockerHubSearchPage struct {
	Results []DockerHubRepo
	Page    int
	HasMore bool
}

// DockerHubDetails is the rich metadata for the selected repository.
type DockerHubDetails struct {
	Name           string
	Description    string
	Overview       string
	Stars          int
	Pulls          int64
	Official       bool
	Automated      bool
	LastUpdated    time.Time
	DateRegistered time.Time
	Categories     []string
	Status         string
	Tag            string
	TagSize        int64
	Architectures  []string
	OS             []string
}

// DockerHubTag is a repository tag from Hub (recent tags list).
type DockerHubTag struct {
	Name        string
	Size        int64
	LastUpdated time.Time
}

type hubSearchResponse struct {
	Count   int    `json:"count"`
	Next    string `json:"next"`
	Results []struct {
		RepoName         string `json:"repo_name"`
		ShortDescription string `json:"short_description"`
		StarCount        int    `json:"star_count"`
		PullCount        int64  `json:"pull_count"`
		IsOfficial       bool   `json:"is_official"`
		IsAutomated      bool   `json:"is_automated"`
	} `json:"results"`
}

type hubRepoResponse struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Description     string `json:"description"`
	FullDescription string `json:"full_description"`
	StarCount       int    `json:"star_count"`
	PullCount       int64  `json:"pull_count"`
	IsAutomated     bool   `json:"is_automated"`
	StatusDesc      string `json:"status_description"`
	LastUpdated     string `json:"last_updated"`
	DateRegistered  string `json:"date_registered"`
	Categories      []struct {
		Name string `json:"name"`
	} `json:"categories"`
}

type hubTagsResponse struct {
	Results []struct {
		Name        string `json:"name"`
		FullSize    int64  `json:"full_size"`
		LastUpdated string `json:"last_updated"`
		Images      []struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
			Size         int64  `json:"size"`
		} `json:"images"`
	} `json:"results"`
}

var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
var mdImageRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)
var mdHeadingRe = regexp.MustCompile(`(?m)^#{1,6}\s*`)
var mdCodeFenceRe = regexp.MustCompile("(?s)```.*?```")

// SearchDockerHub queries the first page of the public Hub search API (no auth).
func SearchDockerHub(query string, limit int) ([]DockerHubRepo, error) {
	page, err := SearchDockerHubPage(query, 1, limit)
	if err != nil {
		return nil, err
	}
	return page.Results, nil
}

// SearchDockerHubPage queries a specific page (1-based).
func SearchDockerHubPage(query string, page, pageSize int) (DockerHubSearchPage, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return DockerHubSearchPage{}, fmt.Errorf("busca vazia")
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 15
	}
	u := "https://hub.docker.com/v2/search/repositories/?query=" + url.QueryEscape(query) +
		"&page_size=" + fmt.Sprintf("%d", pageSize) +
		"&page=" + fmt.Sprintf("%d", page)
	var body hubSearchResponse
	if err := hubGetJSON(u, &body); err != nil {
		return DockerHubSearchPage{}, err
	}
	out := make([]DockerHubRepo, 0, len(body.Results))
	for _, r := range body.Results {
		name := strings.TrimSpace(r.RepoName)
		if name == "" {
			continue
		}
		out = append(out, DockerHubRepo{
			Name:        name,
			Description: strings.TrimSpace(r.ShortDescription),
			Stars:       r.StarCount,
			Pulls:       r.PullCount,
			Official:    r.IsOfficial,
			Automated:   r.IsAutomated,
		})
	}
	return DockerHubSearchPage{
		Results: out,
		Page:    page,
		HasMore: strings.TrimSpace(body.Next) != "",
	}, nil
}

// FetchDockerHubDetails loads repository metadata and a representative tag (size/arches).
func FetchDockerHubDetails(repoName string) (DockerHubDetails, error) {
	repoName = stripImageTag(strings.TrimSpace(repoName))
	if repoName == "" {
		return DockerHubDetails{}, fmt.Errorf("repositório vazio")
	}
	ns, name := SplitDockerHubRepo(repoName)
	base := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/", url.PathEscape(ns), url.PathEscape(name))

	var repo hubRepoResponse
	if err := hubGetJSON(base, &repo); err != nil {
		return DockerHubDetails{}, err
	}

	d := DockerHubDetails{
		Name:           FormatDockerHubRepo(ns, name),
		Description:    strings.TrimSpace(repo.Description),
		Overview:       HubOverviewExcerpt(repo.FullDescription, 520),
		Stars:          repo.StarCount,
		Pulls:          repo.PullCount,
		Official:       ns == "library",
		Automated:      repo.IsAutomated,
		Status:         strings.TrimSpace(repo.StatusDesc),
		LastUpdated:    parseHubTime(repo.LastUpdated),
		DateRegistered: parseHubTime(repo.DateRegistered),
	}
	for _, c := range repo.Categories {
		if n := strings.TrimSpace(c.Name); n != "" {
			d.Categories = append(d.Categories, n)
		}
	}

	tag, err := fetchHubTagInfo(ns, name)
	if err == nil {
		d.Tag = tag.Name
		d.TagSize = tag.FullSize
		d.Architectures = tag.Architectures
		d.OS = tag.OS
		if d.LastUpdated.IsZero() {
			d.LastUpdated = tag.LastUpdated
		}
	}
	return d, nil
}

type hubTagInfo struct {
	Name          string
	FullSize      int64
	LastUpdated   time.Time
	Architectures []string
	OS            []string
}

// ListDockerHubTags returns recent tags for a repository (newest first).
func ListDockerHubTags(repoName string, limit int) ([]DockerHubTag, error) {
	repoName = stripImageTag(strings.TrimSpace(repoName))
	if repoName == "" {
		return nil, fmt.Errorf("repositório vazio")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	ns, name := SplitDockerHubRepo(repoName)
	u := fmt.Sprintf(
		"https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=%d&ordering=last_updated",
		url.PathEscape(ns), url.PathEscape(name), limit,
	)
	var body hubTagsResponse
	if err := hubGetJSON(u, &body); err != nil {
		return nil, err
	}
	out := make([]DockerHubTag, 0, len(body.Results))
	seen := map[string]bool{}
	for _, r := range body.Results {
		tag := strings.TrimSpace(r.Name)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, DockerHubTag{
			Name:        tag,
			Size:        r.FullSize,
			LastUpdated: parseHubTime(r.LastUpdated),
		})
	}
	return out, nil
}

// ImageRepoName strips tag/digest from an image reference.
func ImageRepoName(image string) string {
	return stripImageTag(strings.TrimSpace(image))
}

// WithImageTag returns repo:tag (tag empty → bare repo).
func WithImageTag(repo, tag string) string {
	repo = ImageRepoName(repo)
	tag = strings.TrimSpace(tag)
	if repo == "" {
		return tag
	}
	if tag == "" {
		return repo
	}
	return repo + ":" + tag
}

func fetchHubTagInfo(ns, name string) (hubTagInfo, error) {
	// Prefer "latest"; fall back to most recently updated tag.
	urls := []string{
		fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=1&name=latest",
			url.PathEscape(ns), url.PathEscape(name)),
		fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=1&ordering=last_updated",
			url.PathEscape(ns), url.PathEscape(name)),
	}
	var lastErr error
	for _, u := range urls {
		var body hubTagsResponse
		if err := hubGetJSON(u, &body); err != nil {
			lastErr = err
			continue
		}
		if len(body.Results) == 0 {
			continue
		}
		r := body.Results[0]
		info := hubTagInfo{
			Name:        strings.TrimSpace(r.Name),
			FullSize:    r.FullSize,
			LastUpdated: parseHubTime(r.LastUpdated),
		}
		seenArch := map[string]bool{}
		seenOS := map[string]bool{}
		for _, img := range r.Images {
			arch := strings.TrimSpace(img.Architecture)
			if arch != "" && arch != "unknown" && !seenArch[arch] {
				seenArch[arch] = true
				info.Architectures = append(info.Architectures, arch)
			}
			osName := strings.TrimSpace(img.OS)
			if osName != "" && osName != "unknown" && !seenOS[osName] {
				seenOS[osName] = true
				info.OS = append(info.OS, osName)
			}
		}
		if info.Name != "" {
			return info, nil
		}
	}
	if lastErr != nil {
		return hubTagInfo{}, lastErr
	}
	return hubTagInfo{}, fmt.Errorf("sem tags")
}

func hubGetJSON(u string, dest any) error {
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker hub HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

// SplitDockerHubRepo returns namespace and name (official images use library/).
func SplitDockerHubRepo(repo string) (namespace, name string) {
	repo = stripImageTag(strings.TrimSpace(repo))
	repo = strings.TrimPrefix(repo, "library/")
	if repo == "" {
		return "", ""
	}
	if !strings.Contains(repo, "/") {
		return "library", repo
	}
	parts := strings.SplitN(repo, "/", 2)
	return parts[0], parts[1]
}

// FormatDockerHubRepo formats namespace/name for display (drops library/).
func FormatDockerHubRepo(namespace, name string) string {
	if namespace == "" || namespace == "library" {
		return name
	}
	return namespace + "/" + name
}

// DockerHubURL returns the public Hub page for a repository name.
func DockerHubURL(name string) string {
	name = stripImageTag(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	ns, repo := SplitDockerHubRepo(name)
	if ns == "library" {
		return "https://hub.docker.com/_/" + url.PathEscape(repo)
	}
	return "https://hub.docker.com/r/" + ns + "/" + repo
}

func stripImageTag(name string) string {
	if i := strings.Index(name, "@"); i >= 0 {
		name = name[:i]
	}
	if i := strings.Index(name, ":"); i >= 0 {
		name = name[:i]
	}
	return name
}

func parseHubTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// HubOverviewExcerpt cleans Hub README markdown into a short plain-text blurb.
func HubOverviewExcerpt(md string, maxRunes int) string {
	md = strings.TrimSpace(md)
	if md == "" {
		return ""
	}
	md = mdCodeFenceRe.ReplaceAllString(md, " ")
	md = mdImageRe.ReplaceAllString(md, " ")
	md = mdLinkRe.ReplaceAllString(md, "$1")
	md = mdHeadingRe.ReplaceAllString(md, "")
	md = strings.ReplaceAll(md, "*", "")
	md = strings.ReplaceAll(md, "`", "")
	md = strings.Join(strings.Fields(md), " ")
	if maxRunes <= 0 {
		maxRunes = 400
	}
	runes := []rune(md)
	if len(runes) > maxRunes {
		md = string(runes[:maxRunes])
		if i := strings.LastIndexAny(md, ".!?;"); i > maxRunes/2 {
			md = md[:i+1]
		} else {
			md = strings.TrimSpace(md) + "…"
		}
	}
	return strings.TrimSpace(md)
}

// FormatHubCount formats large Hub counters (pulls/stars).
func FormatHubCount(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 10_000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	case n < 1_000_000:
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	case n < 10_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n < 1_000_000_000:
		return fmt.Sprintf("%.0fM", float64(n)/1_000_000)
	case n < 10_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	default:
		return fmt.Sprintf("%.0fB", float64(n)/1_000_000_000)
	}
}

// FormatHubBytes formats compressed image size.
func FormatHubBytes(n int64) string {
	if n <= 0 {
		return "—"
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < mb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	case n < gb:
		if n >= 100*mb {
			return fmt.Sprintf("%.0f MB", float64(n)/mb)
		}
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/gb)
	}
}

// FormatHubRelative returns a short relative time in Portuguese.
func FormatHubRelative(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%d d", int(d.Hours()/24))
	case d < 24*365*time.Hour:
		return fmt.Sprintf("%d mês", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%d a", int(d.Hours()/(24*365)))
	}
}
