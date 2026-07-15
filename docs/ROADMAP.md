# DevScope — Roadmap

> Última atualização: alinhado ao estado real do código.

---

## MVP (v0.0.1) — Fundação ✅

**Objetivo:** Provar o conceito. TUI funcional com descoberta básica e dashboard.

### Entregas

- [x] Arquitetura e documentação
- [x] Binário Go compilável (`make build`)
- [x] CLI com Cobra (`devscope`, `devscope version`)
- [x] Config via Viper (YAML + env + flags)
- [x] Scanner de diretórios com marcadores básicos
- [x] Detectores: Node, Go, Python, PHP, Docker-only
- [x] Dashboard TUI listando projetos
- [x] Métricas de host (CPU, RAM, Disk) via `/proc`
- [x] Status básico: Running/Stopped via docker-compose e PM2 discovery
- [x] Navegação: Enter (abrir), Esc (voltar), Q (sair), / (filtro)
- [x] Painel de projeto com tabs: Overview, Git
- [x] Git info: branch, last commit, modified files
- [x] CI básico (test + build)

---

## v0.1 — Serviços ✅

**Objetivo:** Integrar Docker e PM2. Projeto mostra containers e workers reais.

### Entregas

- [x] Collector Docker (`docker ps`, inspect)
- [x] Collector Docker stats (`docker stats --no-stream`)
- [x] Collector PM2 (`pm2 jlist` → Workers)
- [x] Correlação container→projeto por mount path
- [x] Tab Containers no painel do projeto
- [x] Métricas por projeto (soma CPU/RAM dos containers)
- [x] Detectores: NestJS, Laravel, Django, Next.js, Vue, React, Nuxt
- [x] Portas abertas via containers + `/proc/net/tcp`
- [x] Status real: Running/Stopped/Degraded
- [x] Keybinding restart/pause/remove container
- [x] Keybinding shell (docker exec / bash)
- [x] GoReleaser + GitHub Releases
- [x] Script de instalação (`curl | bash`)

---

## v0.2 — Observabilidade ✅

**Objetivo:** Logs, health checks e métricas em tempo real.

### Entregas

- [x] Log streaming: Docker follow/pause
- [x] Health checks: HTTP, HTTPS, TCP
- [x] SSL: dias restantes (Let's Encrypt)
- [x] Collector Nginx (vhosts, proxy_pass)
- [x] Tab Logs e Tab Health no painel
- [x] Keybindings: L (logs tab), H (health), O (browser)
- [x] Detectores: Nuxt, Angular, Rust, Spring (via registry)

---

## v0.3 — Polimento ✅

**Objetivo:** UX de produção. Performance e estabilidade.

### Entregas

- [x] Temas (dark/light/auto) via Lip Gloss
- [x] Statusbar com atalhos contextuais
- [x] Help screen (`?`)
- [x] Deploy detection (`deploy.sh`, Makefile, package.json scripts)
- [x] Keybinding `D` (deploy) com confirmação
- [x] Keybinding `L` (LazyGit)
- [x] Fuzzy finder global (`Ctrl+P`)
- [x] Confirmação antes de remove container
- [x] Cache de scan por mtime
- [x] Projetos pinned (config)
- [x] README completo + CONTRIBUTING + CHANGELOG

### Pendente v0.3

- [ ] Homebrew tap via GoReleaser
- [ ] Benchmark formal de memória

---

## v1.0 — Produção (em progresso)

**Objetivo:** Ferramenta confiável para uso diário em VPS.

### Entregas

- [x] amd64 + arm64 testados em CI
- [x] Config file documentado
- [x] Modo `--scan-only` (JSON output, sem TUI)
- [x] Modo `--watch` (atualiza terminal sem TUI)
- [x] Script de instalação (`curl | bash`)
- [ ] Plugin API documentada para detectores custom
- [ ] Collector systemd
- [ ] Supervisor, Traefik, Caddy collectors
- [ ] Last deploy detection (git log, docker image date)
- [ ] Performance: scan de 1000+ diretórios em <5s (benchmark)
- [ ] Cobertura de testes >70%
- [ ] README com GIF demo
- [ ] Todas as distros (Ubuntu, Debian, CentOS, Rocky, AlmaLinux)

---

## v2.0 — Plataforma (planejado)

**Objetivo:** DevScope como hub central de operações de projetos.

### Entregas

- [ ] Multi-host via SSH (lista de VPS, switch entre hosts)
- [ ] Histórico de deploys e eventos
- [ ] Alertas configuráveis (CPU >90%, SSL <7 dias, health fail)
- [ ] Export de relatório (JSON, HTML)
- [ ] Webhook notifications (Slack, Discord, Telegram)
- [ ] Plugin marketplace (detectores da comunidade)
- [ ] Modo servidor (API REST + TUI client remoto)
- [ ] Integração com GitHub Actions / GitLab CI status
- [ ] Comparador de estado entre hosts
- [ ] Backup/restore de configuração
- [ ] Podman nativo (sem compat layer Docker)
- [ ] Suporte a monorepos complexos (Nx, Turborepo, Bazel)

---

## Timeline Estimada

| Versão | Prazo estimado | Foco |
|--------|---------------|------|
| MVP    | ✅ Concluído  | Fundação + TUI |
| v0.1   | ✅ Concluído  | Docker + PM2 + stats |
| v0.2   | ✅ Concluído  | Logs + Health + SSL |
| v0.3   | ✅ Concluído  | UX + Performance |
| v1.0   | Em progresso  | Produção + GIF |
| v2.0   | Planejado     | Plataforma |

---

## Critérios de Release

1. Todos os testes passam
2. Binário compila para linux/amd64 e linux/arm64
3. CHANGELOG atualizado
4. Tag semver criada
5. GitHub Release com binários e checksums
