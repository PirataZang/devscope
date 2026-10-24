package devscopeutil

import "testing"

func TestMaskLine(t *testing.T) {
	cases := []struct{ in, want string }{
		// .env de Laravel
		{"APP_KEY=base64:Zm9vYmFyYmF6cXV4", "APP_KEY=ba••••••••••"},
		{"DB_PASSWORD=s3cr3t", "DB_PASSWORD=s3••••"},
		{`DB_PASSWORD="minha senha"`, "DB_PASSWORD=mi•••••••••"},
		{"DB_PASSWORD=", "DB_PASSWORD="},
		{"DB_HOST=postgres", "DB_HOST=postgres"},
		{"MAIL_PASSWORD = abc123", "MAIL_PASSWORD = ab••••"},
		// DSN pgsql e PDO
		{"pgsql:host=db;port=5432;dbname=app;user=app;password=hunter22", "pgsql:host=db;port=5432;dbname=app;user=app;password=hu••••••"},
		{"Server=db;Uid=sa;Pwd=Sup3rS3nha;", "Server=db;Uid=sa;Pwd=Su••••••••;"},
		// URL de banco
		{"DATABASE_URL=postgres://app:hunter22@db:5432/app", "DATABASE_URL=postgres://app:••••@db:5432/app"},
		{"connecting to redis://default:p4ss@cache:6379/0", "connecting to redis://default:••••@cache:6379/0"},
		// header
		{"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.e30.x", "Authorization: Bearer ey••••••••••"},
		{`{"Authorization": "Bearer abc.def.ghi"}`, `{"Authorization": "Bearer ab•••••••••"}`},
		// JSON
		{`{"api_key": "abcd1234", "user": "igor"}`, `{"api_key": "ab••••••", "user": "igor"}`},
		// tokens soltos
		{"push com ghp_abcdefghijklmnopqrstuvwxyz0123456789", "push com ghp_••••••••"},
		{"token github_pat_11ABCDEFG0123456789_abc", "token gith••••••••"},
		{"glpat-abcdefghij1234567890 expirou", "glpa•••••••• expirou"},
		{"chave sk-proj-abcdefghijklmnop", "chave sk-p••••••••"},
		{"slack xoxb-1234567890-abcdef", "slack xoxb••••••••"},
		{"aws AKIAIOSFODNN7EXAMPLE ok", "aws AKIA•••••••• ok"},
		// o que não é segredo passa intacto
		{"[2026-03-15 10:00:01] production.ERROR: SQLSTATE[08006] connection refused", "[2026-03-15 10:00:01] production.ERROR: SQLSTATE[08006] connection refused"},
		{"task-runner iniciado", "task-runner iniciado"},
	}
	for _, c := range cases {
		if got := MaskLine(c.in); got != c.want {
			t.Errorf("MaskLine(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}
