package routeutil

import (
	"path/filepath"
	"testing"
)

func TestLaravelPrivateAndChainedRoutes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "artisan"), "#!/usr/bin/env php\n")
	mustWrite(t, filepath.Join(root, "routes", "api.php"), `
Route::get('/public', fn () => null);

Route::middleware('auth:sanctum')->get('/me', fn () => null);

Route::middleware(['auth'])->group(function () {
    Route::get('/private', fn () => null);
    Route::post('/private', fn () => null);
    if (true) {
        Route::delete('/nested', fn () => null);
    }
});

Route::middleware('auth')->apiResource('secrets', SecretController::class);
`)

	routes, err := laravelScanner{}.Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	must := []struct {
		m, p string
		auth bool
	}{
		{"GET", "/public", false},
		{"GET", "/me", true},
		{"GET", "/private", true},
		{"POST", "/private", true},
		{"DELETE", "/nested", true},
		{"GET", "/secrets", true},
	}
	for _, want := range must {
		var found *Route
		for i := range routes {
			if routes[i].Method == want.m && normalizePath(routes[i].Path) == normalizePath(want.p) {
				found = &routes[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("missing %s %s in %+v", want.m, want.p, routes)
		}
		if found.Auth != want.auth {
			t.Fatalf("%s %s Auth=%v want %v", want.m, want.p, found.Auth, want.auth)
		}
	}
}
