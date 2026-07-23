package collectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DockerHubRepo struct {
	Name        string
	Description string
	Stars       int
	Official    bool
}

type hubSearchResponse struct {
	Results []struct {
		RepoName         string `json:"repo_name"`
		ShortDescription string `json:"short_description"`
		StarCount        int    `json:"star_count"`
		IsOfficial       bool   `json:"is_official"`
	} `json:"results"`
}

// SearchDockerHub queries the public Hub search API (no auth).
func SearchDockerHub(query string, limit int) ([]DockerHubRepo, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("busca vazia")
	}
	if limit <= 0 {
		limit = 15
	}
	u := "https://hub.docker.com/v2/search/repositories/?query=" + url.QueryEscape(query) +
		"&page_size=" + fmt.Sprintf("%d", limit)
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker hub HTTP %d", resp.StatusCode)
	}
	var body hubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
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
			Official:    r.IsOfficial,
		})
	}
	return out, nil
}
