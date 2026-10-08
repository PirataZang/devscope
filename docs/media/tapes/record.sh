#!/usr/bin/env bash
# Regrava os GIFs e vídeos do README (docs/media/*.gif e *.mp4).
#
#   docs/media/tapes/record.sh              # overview, git e docker
#   docs/media/tapes/record.sh git          # só um
#
# Precisa só de Docker, ffmpeg e python3 com Pillow. O VHS roda num container
# (com git e docker CLI dentro) contra projetos de mentira criados em $DEMO —
# nada da sua máquina aparece na gravação.
#
# Os zooms (<nome>.json) são por tempo. Deixe a máquina quieta enquanto grava:
# com CPU disputada o VHS perde quadros, o vídeo encolhe e o zoom cai na tela
# errada. Ajustou uma tela? Confira com `zoom.py ... --preview` e mexa no json.
set -euo pipefail

TAPES="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$TAPES/../../.." && pwd)"
MEDIA="$ROOT/docs/media"
DEMO="${DEMO:-/tmp/devscope-demo}"
WORK="$(mktemp -d)"
VERSION="$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo dev)"

echo "==> binário $VERSION"
docker run --rm -u "$(id -u):$(id -g)" -e HOME=/tmp -e GOCACHE=/tmp/gc -e GOPATH=/tmp/gp -e CGO_ENABLED=0 \
	-v "$ROOT":/src -v "$WORK":/out -w /src golang:1.24 \
	go build -buildvcs=false -ldflags "-s -w -X github.com/devscope/devscope/pkg/version.Version=$VERSION" \
	-o /out/devscope ./cmd/devscope

echo "==> imagem do VHS"
docker build -q -t devscope-vhs - >/dev/null <<'EOF'
FROM docker:29-cli AS cli
FROM ghcr.io/charmbracelet/vhs:latest
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=cli /usr/local/libexec/docker/cli-plugins /usr/local/libexec/docker/cli-plugins
EOF

echo "==> projetos de demonstração em $DEMO"
for p in loja-api blog; do docker compose -p "$p" down -v >/dev/null 2>&1 || true; done
rm -rf "$DEMO" && mkdir -p "$DEMO/projetos" "$DEMO/.config/devscope" "$DEMO/remotes"
export HOME="$DEMO" GIT_CONFIG_NOSYSTEM=1
git config --global user.name "Ana Souza"
git config --global user.email ana@exemplo.dev
git config --global init.defaultBranch main
git config --global --add safe.directory '*'
printf 'scan:\n  paths: [%s/projetos]\n  max_depth: 3\n' "$DEMO" >"$DEMO/.config/devscope/config.yaml"

# commit <mensagem> <autor> <dias atrás>
commit() {
	local d
	d="$(date -d "$3 days ago" -R)"
	GIT_AUTHOR_NAME="$2" GIT_COMMITTER_NAME="$2" GIT_AUTHOR_DATE="$d" GIT_COMMITTER_DATE="$d" git commit -qm "$1"
}
put() { mkdir -p "$(dirname "$1")" && echo "$2" >>"$1" && git add -A; }

P="$DEMO/projetos/loja-api" && mkdir -p "$P" && cd "$P" && git init -q
cat >package.json <<'EOF'
{ "name": "loja-api", "version": "2.4.0", "scripts": { "start": "node src/server.js" },
  "dependencies": { "express": "^4.19.2", "pg": "^8.11.0", "ioredis": "^5.4.0" } }
EOF
cat >docker-compose.yml <<'EOF'
services:
  web:
    image: nginx:alpine
    ports: ["8080:80"]
    restart: unless-stopped
    volumes: ["./public:/usr/share/nginx/html:ro"]
  cache:
    image: redis:7-alpine
    ports: ["6379:6379"]
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: loja
      POSTGRES_PASSWORD: s3nh4-sup3r-secreta
      POSTGRES_DB: loja
EOF
echo "node_modules/" >.gitignore
put src/server.js "const express = require('express');" && commit "chore: estrutura inicial da API" "Ana Souza" 20
put src/routes/produtos.js "module.exports = router => router.get('/produtos', listar);" && commit "feat: rota de listagem de produtos" "Bruno Lima" 17
put src/routes/pedidos.js "// pedidos" && commit "feat: criação de pedidos" "Ana Souza" 14
put src/services/cache.js "// cache" && commit "perf: cache de catálogo no redis" "Carla Mendes" 11
git tag v2.3.0
git checkout -qb feat/pagamento-pix
put src/services/pix.js "// pix" && commit "feat: integração com PIX" "Bruno Lima" 9
put src/routes/webhook.js "// webhook" && commit "feat: webhook de confirmação do PIX" "Bruno Lima" 7
git checkout -q main && git checkout -qb hotfix/estoque
put src/routes/pedidos.js "// estoque" && commit "fix: baixa de estoque duplicada" "Carla Mendes" 6
git checkout -q main
git merge -q --no-ff hotfix/estoque -m "Merge branch 'hotfix/estoque'"
git merge -q --no-ff feat/pagamento-pix -m "Merge branch 'feat/pagamento-pix'"
git tag v2.4.0
git init -q --bare "$DEMO/remotes/loja-api.git"
git remote add origin "$DEMO/remotes/loja-api.git" && git push -q -u origin main 2>/dev/null
git remote set-url origin https://github.com/exemplo/loja-api.git
put src/routes/produtos.js "// validação" && commit "fix: valida preço negativo" "Ana Souza" 2
put src/server.js "// log" && commit "feat: log estruturado das requisições" "Ana Souza" 1
git checkout -qb feat/cupons
put src/services/cupons.js "// cupons" && commit "feat: cupons de desconto" "Diego Rocha" 1
git checkout -q main
echo "// rascunho" >>src/services/cache.js && git stash -q
echo "app.use(cors());" >>src/server.js
echo "// TODO limite" >>src/routes/pedidos.js
echo "SELECT 1;" >src/seed.sql
mkdir -p public/api && echo '<h1>Loja API</h1>' >public/index.html
echo ok >public/health && echo '{"status":"ok"}' >public/api/health
echo "public/" >>.git/info/exclude
# um de cada vez: a lista de containers ordena pelo mais novo (web, db, cache)
for svc in cache db web; do docker compose -p loja-api up -d --quiet-pull "$svc" 2>/dev/null && sleep 1; done

P="$DEMO/projetos/painel-web" && mkdir -p "$P" && cd "$P" && git init -q
echo '{ "name": "painel-web", "dependencies": { "vue": "^3.4.0", "vite": "^5.0.0" } }' >package.json
put src/App.vue "<template/>" && commit "feat: dashboard de vendas" "Carla Mendes" 3

P="$DEMO/projetos/frete-service" && mkdir -p "$P" && cd "$P" && git init -q
printf 'module frete\n\ngo 1.24\n' >go.mod
put main.go "package main" && commit "feat: cálculo de frete" "Diego Rocha" 4
git checkout -qb feat/correios
put main.go "// correios" && commit "wip: api dos correios" "Diego Rocha" 1

P="$DEMO/projetos/blog" && mkdir -p "$P" && cd "$P" && git init -q
printf 'services:\n  site:\n    image: nginx:alpine\n    ports: ["8081:80"]\n' >docker-compose.yml
git add -A && commit "chore: blog estático" "Ana Souza" 30
docker compose -p blog up -d --quiet-pull 2>/dev/null && docker compose -p blog stop 2>/dev/null

cp "$TAPES"/*.tape "$WORK/"
NAMES=("$@") && [ $# -gt 0 ] || NAMES=(overview git docker)
for n in "${NAMES[@]}"; do
	echo "==> gravando $n"
	docker run --rm -u "$(id -u):$(id -g)" --group-add "$(stat -c %g /var/run/docker.sock)" \
		--network host --hostname devbox \
		-v /var/run/docker.sock:/var/run/docker.sock -v "$DEMO:$DEMO" -v "$WORK:/vhs" \
		-v "$WORK/devscope:/usr/local/bin/devscope" -e HOME="$DEMO" -w "$DEMO" \
		devscope-vhs "/vhs/$n.tape" >/dev/null
	python3 "$TAPES/zoom.py" "$WORK/$n.mp4" "$TAPES/$n.json" "$MEDIA/$n"
	ls -lh "$MEDIA/$n.gif" "$MEDIA/$n.mp4"
done
rm -rf "$WORK"
