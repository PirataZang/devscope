# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/),
e este projeto segue o [Versionamento Semântico](https://semver.org/).

## [Unreleased]

### Added
- **Aba Database (TOOLS)**
  - Descoberta automática de Postgres/MySQL nos containers do projeto
  - Listagem de tabelas/colunas e execução de SQL
  - Histórico de queries e cliente fullscreen
  - Execução cross-platform (Windows e Unix)
- **Aba WebSocket (TOOLS)**
  - Sessões WS com connect/disconnect e lifecycle events
  - Overview em 3 colunas (connections/stats/filters · messages+send · inspector)
  - Sub-abas Messages, Send, History e Settings
  - Filtros por tipo de frame (text/JSON/binary/errors/in/out) e busca
- **Aba Kubernetes (SCOPE)**
  - Cliente estilo LazyDocker via `kubectl`
  - Pods, Deployments, Services e manifests do projeto
  - Apply/edit/delete YAML, logs, scale e troca de namespace/context
- **Aba API (TOOLS)** — cliente HTTP embutido
  - Layout estilo LazyDocker/Postman (Request, URL, Headers, Auth + Body/Response)
  - Métodos GET/POST/PUT/PATCH/DELETE, Bearer/Basic auth
  - Sugestão automática de porta do projeto, histórico de requests e busca na response
- **Aba JSON (UTILS)**
  - Pretty/minify/validate, sort keys, strip nulls
  - Conversão JSON ⇄ YAML/TOML/XML, diff e busca por chave
- **Aba JWT (UTILS)**
  - Decode/verify/generate/sign estilo jwt.io (HS256/384/512)
  - Copy claims e export JSON
- **Aba Rotas (UTILS)**
  - Detecção de stack e discovery de endpoints (OpenAPI + parsers)
  - Suporte a Express, NestJS, Next/Nuxt, FastAPI, Flask, Django, Laravel, Rails, Spring, Go, Rust e outros
  - Abrir rota na aba API com method + URL
- **Aba Ngrok (TOOLS)**
  - Tunnels, requests, history, domains e settings
  - Wizard de criação e agent info por projeto
- **Aba Overview e Metrics**
  - Dashboard de contexto do projeto (env, host, health, recursos)
  - Aba Metrics dedicada
- **Integração OpenCode** — `Shift+O` abre o OpenCode no diretório do projeto
- **Sidebar do projeto** com navegação por grupos (SCOPE → WATCH → TOOLS → UTILS)
- **Collectors e utilitários**
  - `database`, `k8s`, `shell`
  - `jsonutil`, `jwtutil`, `wsutil`, `ngrokutil`, `routeutil`
- Testes unitários para Database, WebSocket, Kubernetes, JSON, JWT, Routes, Ngrok e tabs relacionadas

### Changed
- Melhor tratamento de erros em comandos Docker
- Aba Git e Containers com mais detalhes e navegação aprimorada
- README atualizado com atalhos e funcionalidades das novas abas

### Fixed
- Execução de comandos de database em Windows vs plataformas Unix

## [1.0.0] - 2026-07-17

### Added
- **Aba Git completa**
  - Gerenciamento de branches (checkout, criar, renomear, apagar, marcar origem)
  - Histórico de commits com visualização de detalhes (mensagem e arquivos alterados)
  - Cherry-pick: seleção individual/range de commits, copiar e colar entre branches
  - Pull/Push, merge de branch, abrir Pull Request no GitHub
  - Filtro de branches e working tree
  - Diff colorido (adições/remoções)
- **Aba de Containers**
  - Listagem e monitoramento de containers Docker
  - Detalhes: logs (com follow), stats, env, config
  - Ciclo de vida: start/restart, stop, pause/resume, remover
  - Shell interativo dentro do container
  - Docker compose up/down/restart
- Carregamento assíncrono de detalhes Git/Docker do projeto
- CHANGELOG e documentação de arquitetura

### Fixed
- **Tela de ajuda/atalhos não abria dentro de um projeto**
  - O `?` só funcionava no dashboard; agora abre a ajuda com todos os atalhos
    em qualquer view de projeto. Feche com `esc` ou `?`.

## [0.1.2] - 2026-07-15

### Added
- UI de gerenciamento de containers com logs, métricas e lifecycle handlers
- Integração Git na TUI: branches, histórico de commits e views de detalhe
- Toggle de help/comandos no update de projeto

## [0.1.1] - 2026-07-15

### Added
- Integração Git com navegação de branches, histórico e detalhes de commit
- Validação SHA256 de checksums nos scripts de instalação
- Scripts de instalação cross-platform (bash + PowerShell) com configuração de PATH
- Métricas de disco do host e cross-platform
- Documentação de arquitetura

## [0.1.0] - 2026-07-15

### Added
- Primeira versão tagueada
- TUI dashboard com descoberta automática de projetos
- Detectores de stack (Node, Go, Python, PHP, Docker e frameworks comuns)
- Collectors Docker e PM2
- Métricas de host (CPU, RAM, Disk)
- Health checks HTTP/TCP e SSL
- Collector Nginx
- Tabs Overview, Git, Containers, Logs e Health
- Temas, help screen, fuzzy finder, deploy detection
- GoReleaser + GitHub Releases
- CLI: `devscope`, `scan --json`, `watch`, `version`

[Unreleased]: https://github.com/PirataZang/devscope/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/PirataZang/devscope/compare/v0.1.2...v1.0.0
[0.1.2]: https://github.com/PirataZang/devscope/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/PirataZang/devscope/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/PirataZang/devscope/releases/tag/v0.1.0
