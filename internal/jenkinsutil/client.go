package jenkinsutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ServerInfo struct {
	Connected bool
	Version   string
	Mode      string
	NodeName  string
	URL       string
	User      string
	NumExec   int
	BusyExec  int
	Quieting  bool
	Err       string
}

type Job struct {
	Name        string
	FullName    string
	URL         string
	Color       string
	Status      string // success|failure|unstable|aborted|running|disabled|notbuilt|unknown
	Description string
	LastBuild   int
	InQueue     bool
	Buildable   bool
}

type Build struct {
	Number    int
	URL       string
	Result    string // SUCCESS|FAILURE|UNSTABLE|ABORTED|""
	Building  bool
	Duration  int64 // ms
	Timestamp int64 // ms epoch
	Display   string
	FullName  string // job full name when known
}

type Client struct {
	cfg    ProjectConfig
	http   *http.Client
	crumb  string
	crumbF string
}

func NewClient(cfg ProjectConfig) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (c *Client) Ping() ServerInfo {
	info := ServerInfo{URL: c.cfg.URL, User: c.cfg.User}
	if !c.cfg.Configured() {
		info.Err = "configure URL, user e token em Settings"
		return info
	}
	body, hdr, err := c.get("/api/json?tree=mode,nodeName,numExecutors,quietingDown")
	if err != nil {
		info.Err = err.Error()
		return info
	}
	info.Connected = true
	info.Version = hdr.Get("X-Jenkins")
	var raw struct {
		Mode         string `json:"mode"`
		NodeName     string `json:"nodeName"`
		NumExecutors int    `json:"numExecutors"`
		QuietingDown bool   `json:"quietingDown"`
	}
	_ = json.Unmarshal(body, &raw)
	info.Mode = raw.Mode
	info.NodeName = raw.NodeName
	info.NumExec = raw.NumExecutors
	info.Quieting = raw.QuietingDown
	return info
}

func (c *Client) ListJobs() ([]Job, error) {
	path := "/api/json?tree=jobs[name,url,color,description,buildable,inQueue,lastBuild[number]]"
	if c.cfg.Folder != "" {
		path = jobAPIPath(c.cfg.Folder) + "/api/json?tree=jobs[name,url,color,description,buildable,inQueue,lastBuild[number],fullName]"
	}
	body, _, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Jobs []struct {
			Name        string `json:"name"`
			FullName    string `json:"fullName"`
			URL         string `json:"url"`
			Color       string `json:"color"`
			Description string `json:"description"`
			Buildable   bool   `json:"buildable"`
			InQueue     bool   `json:"inQueue"`
			LastBuild   *struct {
				Number int `json:"number"`
			} `json:"lastBuild"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(raw.Jobs))
	for _, j := range raw.Jobs {
		full := j.FullName
		if full == "" {
			if c.cfg.Folder != "" {
				full = strings.Trim(c.cfg.Folder, "/") + "/" + j.Name
			} else {
				full = j.Name
			}
		}
		last := 0
		if j.LastBuild != nil {
			last = j.LastBuild.Number
		}
		out = append(out, Job{
			Name:        j.Name,
			FullName:    full,
			URL:         j.URL,
			Color:       j.Color,
			Status:      ColorToStatus(j.Color),
			Description: j.Description,
			LastBuild:   last,
			InQueue:     j.InQueue,
			Buildable:   j.Buildable,
		})
	}
	return out, nil
}

func (c *Client) GetJob(fullName string) (Job, []Build, error) {
	path := jobAPIPath(fullName) + "/api/json?tree=name,fullName,url,color,description,buildable,inQueue,lastBuild[number],builds[number,url,result,building,duration,timestamp,displayName]{0,20}"
	body, _, err := c.get(path)
	if err != nil {
		return Job{}, nil, err
	}
	var raw struct {
		Name        string `json:"name"`
		FullName    string `json:"fullName"`
		URL         string `json:"url"`
		Color       string `json:"color"`
		Description string `json:"description"`
		Buildable   bool   `json:"buildable"`
		InQueue     bool   `json:"inQueue"`
		LastBuild   *struct {
			Number int `json:"number"`
		} `json:"lastBuild"`
		Builds []struct {
			Number      int    `json:"number"`
			URL         string `json:"url"`
			Result      string `json:"result"`
			Building    bool   `json:"building"`
			Duration    int64  `json:"duration"`
			Timestamp   int64  `json:"timestamp"`
			DisplayName string `json:"displayName"`
		} `json:"builds"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Job{}, nil, err
	}
	full := raw.FullName
	if full == "" {
		full = fullName
	}
	last := 0
	if raw.LastBuild != nil {
		last = raw.LastBuild.Number
	}
	job := Job{
		Name:        raw.Name,
		FullName:    full,
		URL:         raw.URL,
		Color:       raw.Color,
		Status:      ColorToStatus(raw.Color),
		Description: raw.Description,
		LastBuild:   last,
		InQueue:     raw.InQueue,
		Buildable:   raw.Buildable,
	}
	builds := make([]Build, 0, len(raw.Builds))
	for _, b := range raw.Builds {
		builds = append(builds, Build{
			Number:    b.Number,
			URL:       b.URL,
			Result:    b.Result,
			Building:  b.Building,
			Duration:  b.Duration,
			Timestamp: b.Timestamp,
			Display:   b.DisplayName,
			FullName:  full,
		})
	}
	return job, builds, nil
}

func (c *Client) GetBuild(fullName string, number int) (Build, error) {
	path := jobAPIPath(fullName) + "/" + strconv.Itoa(number) + "/api/json?tree=number,url,result,building,duration,timestamp,displayName,fullDisplayName"
	body, _, err := c.get(path)
	if err != nil {
		return Build{}, err
	}
	var raw struct {
		Number          int    `json:"number"`
		URL             string `json:"url"`
		Result          string `json:"result"`
		Building        bool   `json:"building"`
		Duration        int64  `json:"duration"`
		Timestamp       int64  `json:"timestamp"`
		DisplayName     string `json:"displayName"`
		FullDisplayName string `json:"fullDisplayName"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Build{}, err
	}
	return Build{
		Number:    raw.Number,
		URL:       raw.URL,
		Result:    raw.Result,
		Building:  raw.Building,
		Duration:  raw.Duration,
		Timestamp: raw.Timestamp,
		Display:   firstNonEmpty(raw.FullDisplayName, raw.DisplayName),
		FullName:  fullName,
	}, nil
}

func (c *Client) BuildConsole(fullName string, number int) (string, error) {
	path := jobAPIPath(fullName) + "/" + strconv.Itoa(number) + "/consoleText"
	body, _, err := c.get(path)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) TriggerBuild(fullName string) error {
	if err := c.ensureCrumb(); err != nil {
		return err
	}
	path := jobAPIPath(fullName) + "/build"
	return c.post(path, nil)
}

func (c *Client) StopBuild(fullName string, number int) error {
	if err := c.ensureCrumb(); err != nil {
		return err
	}
	path := jobAPIPath(fullName) + "/" + strconv.Itoa(number) + "/stop"
	return c.post(path, nil)
}

func (c *Client) QueueDepth() (int, error) {
	body, _, err := c.get("/queue/api/json?tree=items[id]")
	if err != nil {
		return 0, err
	}
	var raw struct {
		Items []struct {
			ID int `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, err
	}
	return len(raw.Items), nil
}

func ColorToStatus(color string) string {
	c := strings.ToLower(color)
	switch {
	case strings.HasSuffix(c, "_anime") || strings.Contains(c, "anime"):
		return "running"
	case c == "blue" || c == "green":
		return "success"
	case c == "red":
		return "failure"
	case c == "yellow":
		return "unstable"
	case c == "aborted":
		return "aborted"
	case c == "disabled" || c == "notbuilt" || c == "grey" || c == "nobuilt":
		return "disabled"
	default:
		if c == "" {
			return "unknown"
		}
		return c
	}
}

func BuildStatus(b Build) string {
	if b.Building {
		return "running"
	}
	switch strings.ToUpper(b.Result) {
	case "SUCCESS":
		return "success"
	case "FAILURE":
		return "failure"
	case "UNSTABLE":
		return "unstable"
	case "ABORTED":
		return "aborted"
	default:
		return "unknown"
	}
}

func FormatDuration(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	sec := ms / 1000
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	m := sec / 60
	s := sec % 60
	if m < 60 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh%02dm", h, m)
}

func FormatAgo(tsMs int64) string {
	if tsMs <= 0 {
		return "—"
	}
	t := time.UnixMilli(tsMs)
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("%d min atrás", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh atrás", int(d.Hours()))
	default:
		return t.Format("02/01 15:04")
	}
}

func jobAPIPath(fullName string) string {
	fullName = strings.Trim(fullName, "/")
	if fullName == "" {
		return ""
	}
	parts := strings.Split(fullName, "/")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString("/job/")
		b.WriteString(url.PathEscape(p))
	}
	return b.String()
}

func (c *Client) ensureCrumb() error {
	if c.crumb != "" {
		return nil
	}
	body, _, err := c.get("/crumbIssuer/api/json")
	if err != nil {
		// some Jenkins instances disable crumbs
		return nil
	}
	var raw struct {
		Crumb             string `json:"crumb"`
		CrumbRequestField string `json:"crumbRequestField"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	c.crumb = raw.Crumb
	c.crumbF = raw.CrumbRequestField
	if c.crumbF == "" {
		c.crumbF = "Jenkins-Crumb"
	}
	return nil
}

func (c *Client) get(path string) ([]byte, http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, c.cfg.URL+path, nil)
	if err != nil {
		return nil, nil, err
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.Header, err
	}
	if resp.StatusCode >= 400 {
		return nil, resp.Header, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateErr(string(b)))
	}
	return b, resp.Header, nil
}

func (c *Client) post(path string, form url.Values) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(http.MethodPost, c.cfg.URL+path, body)
	if err != nil {
		return err
	}
	c.auth(req)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.crumb != "" {
		req.Header.Set(c.crumbF, c.crumb)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 && resp.StatusCode != 201 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateErr(string(b)))
	}
	return nil
}

func (c *Client) auth(req *http.Request) {
	if c.cfg.User != "" {
		req.SetBasicAuth(c.cfg.User, c.cfg.Token)
	}
}

func truncateErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
