package nginxutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProject(t *testing.T, mainConf string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.conf"), []byte(mainConf), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sites"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func singleConf(name, serverName, target, root string) NewConf {
	return NewConf{Name: name, Kind: KindSingle, NewSite: NewSite{Name: name, ServerName: serverName, Target: target, Root: root}}
}

func TestDetectAndDiscover(t *testing.T) {
	dir := writeProject(t, "include sites/*.inc;\n")
	if !Detect(dir) {
		t.Fatal("expected nginx project detected")
	}
	proxy := `server {
    listen 80;
    server_name api.example.com;
    location / {
        proxy_pass http://127.0.0.1:3000;
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "sites", "api.inc"), []byte(proxy), 0o644); err != nil {
		t.Fatal(err)
	}
	static := `server {
    listen 443 ssl;
    server_name static.example.com;
    root /var/www/static;
}
`
	if err := os.WriteFile(filepath.Join(dir, "sites", "static.inc"), []byte(static), 0o644); err != nil {
		t.Fatal(err)
	}
	layout, sites, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if layout.Ext != ".inc" {
		t.Fatalf("ext=%q", layout.Ext)
	}
	if len(sites) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(sites), sites)
	}
	if sites[0].Name != "api" || sites[0].ProxyPass != "http://127.0.0.1:3000" || sites[0].SSL || sites[0].Kind != KindSingle {
		t.Fatalf("api site: %+v", sites[0])
	}
	if sites[1].Name != "static" || sites[1].Root != "/var/www/static" || !sites[1].SSL || sites[1].Kind != KindSingle {
		t.Fatalf("static site: %+v", sites[1])
	}
}

func TestCreateConfSingleWritesFileAndSkipsIncludeWhenGlobPresent(t *testing.T) {
	dir := writeProject(t, "http {\n    include sites/*.inc;\n}\n")
	before, _ := os.ReadFile(filepath.Join(dir, "main.conf"))

	site, err := CreateConf(dir, singleConf("My API!", "api.example.com", "http://127.0.0.1:4000", ""))
	if err != nil {
		t.Fatal(err)
	}
	if site.Name != "my-api" || site.Kind != KindSingle {
		t.Fatalf("site=%+v, want sanitized single", site)
	}
	got, err := os.ReadFile(filepath.Join(dir, "sites", "my-api.inc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "proxy_pass http://127.0.0.1:4000;") {
		t.Fatalf("missing proxy_pass: %s", got)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "main.conf"))
	if string(before) != string(after) {
		t.Fatalf("main.conf should stay untouched when glob include already covers sites/: %s", after)
	}
}

func TestCreateConfAddsIncludeWhenMissing(t *testing.T) {
	// main.conf sem include nenhum: só acha a pasta de nível 1 (nome
	// qualquer, não "sites") pelo conteúdo de um arquivo já existente nela.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.conf"), []byte("http {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vhosts := filepath.Join(dir, "vhosts")
	if err := os.MkdirAll(vhosts, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "server {\n    listen 80;\n    server_name existing.example.com;\n    proxy_pass http://127.0.0.1:9000;\n}\n"
	if err := os.WriteFile(filepath.Join(vhosts, "existing.inc"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := CreateConf(dir, singleConf("static", "x.example.com", "", "/var/www/x")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(vhosts, "static.inc")); err != nil {
		t.Fatalf("expected file created inside arbitrarily-named dir: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "main.conf"))
	if !strings.Contains(string(after), "include vhosts/*.inc;") {
		t.Fatalf("expected include appended for the actual dir name: %s", after)
	}
}

func TestCreateConfRejectsDuplicateAndMissingTarget(t *testing.T) {
	dir := writeProject(t, "include sites/*.inc;\n")
	if _, err := CreateConf(dir, singleConf("api", "api.example.com", "", "")); err == nil {
		t.Fatal("expected error: no target/root")
	}
	if _, err := CreateConf(dir, singleConf("api", "api.example.com", "http://x", "")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateConf(dir, singleConf("api", "api.example.com", "http://x", "")); err == nil {
		t.Fatal("expected error: duplicate site")
	}
}

func TestDeleteSiteRejectsPathEscape(t *testing.T) {
	dir := writeProject(t, "include sites/*.inc;\n")
	if err := DeleteSite(dir, "../outside.conf"); err == nil {
		t.Fatal("expected path traversal rejected")
	}
}

// TestDiscoverAnyDirName garante que a pasta de nível 1 é achada pelo include
// de verdade no main.conf, não por bater com um nome de uma lista fixa.
func TestDiscoverAnyDirName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.conf"), []byte("include domains-i-manage/*.conf;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(dir, "domains-i-manage")
	if err := os.MkdirAll(custom, 0o755); err != nil {
		t.Fatal(err)
	}
	site := "server {\n    listen 80;\n    server_name weird.example.com;\n    proxy_pass http://127.0.0.1:5000;\n}\n"
	if err := os.WriteFile(filepath.Join(custom, "weird.conf"), []byte(site), 0o644); err != nil {
		t.Fatal(err)
	}
	layout, sites, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if layout.SitesDir != custom || layout.Ext != ".conf" {
		t.Fatalf("layout=%+v", layout)
	}
	if len(sites) != 1 || sites[0].Name != "weird" || sites[0].Kind != KindSingle {
		t.Fatalf("sites=%+v", sites)
	}
}

// TestDiscoverIncludeWithContainerPathFallsBackToBasename cobre o docker
// compose que monta a pasta local dentro de /etc/nginx no container: o
// include no main.conf aponta pro caminho de dentro do container, mas o
// arquivo local mora numa pasta com o mesmo nome final na raiz do projeto.
func TestDiscoverIncludeWithContainerPathFallsBackToBasename(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.conf"), []byte("include /etc/nginx/routes/*.inc;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, "routes")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	site := "server {\n    listen 80;\n    server_name mounted.example.com;\n    proxy_pass http://127.0.0.1:6000;\n}\n"
	if err := os.WriteFile(filepath.Join(local, "mounted.inc"), []byte(site), 0o644); err != nil {
		t.Fatal(err)
	}
	layout, sites, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if layout.SitesDir != local {
		t.Fatalf("layout=%+v, want fallback to local basename dir", layout)
	}
	if len(sites) != 1 || sites[0].Name != "mounted" {
		t.Fatalf("sites=%+v", sites)
	}
}

// TestCreateConfHubThenCreateInc cobre o fluxo novo: cria um .conf hub (só
// pasta + include), depois cadastra .inc dentro dela, e confere que o hub
// aparece em Discover como Kind=hub e as incs aparecem via DiscoverHubIncs.
func TestCreateConfHubThenCreateInc(t *testing.T) {
	dir := writeProject(t, "include sites/*.conf;\n")

	hub, err := CreateConf(dir, NewConf{Name: "apihub", Kind: KindHub, HubDirName: "apihub"})
	if err != nil {
		t.Fatal(err)
	}
	if hub.Kind != KindHub || hub.HubDir == "" {
		t.Fatalf("hub=%+v", hub)
	}
	if _, err := os.Stat(hub.HubDir); err != nil {
		t.Fatalf("hub dir not created: %v", err)
	}

	_, sites, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Kind != KindHub || sites[0].Name != "apihub" {
		t.Fatalf("discovered=%+v", sites)
	}
	rediscoveredHub := sites[0]

	if _, err := CreateInc(dir, rediscoveredHub, NewSite{Name: "users", ServerName: "users.api.example.com", Target: "http://127.0.0.1:7001"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInc(dir, rediscoveredHub, NewSite{Name: "orders", ServerName: "orders.api.example.com", Target: "http://127.0.0.1:7002"}); err != nil {
		t.Fatal(err)
	}

	incs, err := DiscoverHubIncs(dir, rediscoveredHub)
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 2 || incs[0].Name != "orders" || incs[1].Name != "users" {
		t.Fatalf("incs=%+v", incs)
	}
	if incs[1].ProxyPass != "http://127.0.0.1:7001" {
		t.Fatalf("users inc: %+v", incs[1])
	}

	// deletar o hub não deve mexer nas incs dele.
	if err := DeleteSite(dir, hub.File); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(rediscoveredHub.HubDir, "users.inc")); err != nil {
		t.Fatalf("deleting the hub pointer should not delete its incs: %v", err)
	}
}

func TestCreateIncRejectsNonHub(t *testing.T) {
	dir := writeProject(t, "include sites/*.inc;\n")
	single, err := CreateConf(dir, singleConf("api", "api.example.com", "http://x", ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInc(dir, single, NewSite{Name: "x", ServerName: "x.example.com", Target: "http://y"}); err == nil {
		t.Fatal("expected error: not a hub")
	}
}
