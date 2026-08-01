package collectors

import (
	"strings"
	"testing"
	"time"
)

func TestDockerHubURL(t *testing.T) {
	cases := map[string]string{
		"postgres":              "https://hub.docker.com/_/postgres",
		"postgres:16":           "https://hub.docker.com/_/postgres",
		"library/redis":         "https://hub.docker.com/_/redis",
		"itzg/minecraft-server": "https://hub.docker.com/r/itzg/minecraft-server",
		"":                      "",
	}
	for in, want := range cases {
		if got := DockerHubURL(in); got != want {
			t.Fatalf("%q => %q, want %q", in, got, want)
		}
	}
}

func TestSplitDockerHubRepo(t *testing.T) {
	ns, name := SplitDockerHubRepo("postgres:16")
	if ns != "library" || name != "postgres" {
		t.Fatalf("got %s/%s", ns, name)
	}
	ns, name = SplitDockerHubRepo("itzg/minecraft-server")
	if ns != "itzg" || name != "minecraft-server" {
		t.Fatalf("got %s/%s", ns, name)
	}
}

func TestFormatHubCountAndBytes(t *testing.T) {
	if FormatHubCount(428_415_539) != "428M" {
		t.Fatalf("pulls=%q", FormatHubCount(428_415_539))
	}
	if got := FormatHubBytes(162_351_975); got != "155 MB" {
		t.Fatalf("size=%q", got)
	}
	if FormatHubBytes(0) != "—" {
		t.Fatalf("zero size")
	}
}

func TestHubOverviewExcerpt(t *testing.T) {
	md := "# Title\n\nHello [world](https://x.test) and **bold**.\n\n```go\ncode\n```\nMore text here for padding."
	got := HubOverviewExcerpt(md, 80)
	if strings.Contains(got, "http") || strings.Contains(got, "#") || strings.Contains(got, "```") {
		t.Fatalf("markdown left: %q", got)
	}
	if !strings.Contains(got, "Hello world") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHubRelative(t *testing.T) {
	if FormatHubRelative(time.Time{}) != "—" {
		t.Fatal("zero")
	}
	got := FormatHubRelative(time.Now().Add(-3 * time.Hour))
	if got != "3 h" {
		t.Fatalf("got %q", got)
	}
}

func TestWithImageTag(t *testing.T) {
	if got := WithImageTag("node:22", "trixie"); got != "node:trixie" {
		t.Fatalf("got %q", got)
	}
	if got := WithImageTag("library/postgres", "16"); got != "library/postgres:16" {
		t.Fatalf("got %q", got)
	}
	if got := WithImageTag("itzg/minecraft-server:java17", "latest"); got != "itzg/minecraft-server:latest" {
		t.Fatalf("got %q", got)
	}
	if ImageRepoName("node:trixie") != "node" {
		t.Fatal(ImageRepoName("node:trixie"))
	}
}
