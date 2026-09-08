# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/),
e este projeto segue o [Versionamento Semântico](https://semver.org/).

## [Unreleased]

### Added
- **Aba Nginx (TOOLS)** — cadastro de rotas de nginx direto do projeto, sem editar arquivo na mão
  - Detecta o `main.conf`/`nginx.conf` e a pasta de nível 1 (`.conf`) lendo o `include` de verdade — funciona com qualquer nome de pasta, com fallback por conteúdo (varredura de subpastas) e por caminho de dentro do container (docker volume montado com outro nome local)
  - Cada `.conf` de nível 1 é **single** (um `server{}` completo, com `proxy_pass` ou `root`) ou **hub** (um `server{}` de verdade, com seu próprio `listen`/`server_name`/ssl, que só inclui uma pasta de `.inc`)
  - `Enter` num hub abre as `.inc` dele — cada `.inc` é um `location{}` (ou o par redirect+`alias` pra site estático, com `try_files` no padrão SPA), criado/apagado sem sair do hub
  - Wizard de criação com campos que mudam pelo tipo: nome + tipo + server_name + porta + ssl + pasta (hub) · nome + server_name + proxy_pass/root + porta + ssl (single) · path + label opcional + porta/proxy_pass ou dist (`.inc` dentro de um hub)
  - `A` alterna entre ver só as rotas deste projeto ou de todos os projetos abertos no devscope, com a origem marcada por linha
  - Delete com confirmação; apagar um hub não apaga a pasta nem as `.inc` dela
- **Cloudflare Tunnel**: limpeza de túneis órfãos
  - `K` encerra de uma vez todo túnel vivo que não é deste projeto — útil depois de um restart do devscope que deixa o `cloudflared` filho rodando sozinho, ocupando a faixa de porta de métricas (20241+) e travando novos túneis
  - Túneis de outros projetos/do host aparecem por padrão na lista (antes só apertando `A`); `A` agora serve pra filtrar de volta pra só o projeto atual
- **Credenciais manuais de banco** (aba Database) — permite informar host/porta/usuário/senha de um banco que não roda em container Docker local do projeto (salvo em `.devscope/database.json`); `collectors/database.go` roda `psql`/`mysql` direto contra o host quando não há container, além do caminho existente via `docker exec`
- **`docs/DESIGN.md`** — padrão de telas de módulo (cabeçalho de identificação, régua de abas numerada, corpo em painéis, barra de comandos larga no rodapé), extraído das telas já convertidas e usado como referência para toda tela nova ou reformulada
- **Sparkline em Braille** (`spark.go`) — histórico de CPU/RAM/disco do host e ondas de status animadas, compartilhados entre os módulos

### Changed
- `.devscope/` agora é criado por um helper compartilhado (`devscopeutil.EnsureDir`), reaproveitado por `cfutil`, `jenkinsutil`, `ngrokutil`, `sshutil`, `wsutil` e o novo `dbutil`
- `.gitignore` passa a ignorar `.devscope/` (evita versionar config e credenciais locais do projeto)
- **Todas as telas de módulo redesenhadas** para o padrão do `docs/DESIGN.md` — Git, Containers, GH Actions, ngrok, Cloudflare Tunnel, SSH, Kubernetes, Swarm, Nginx, Database, Rotas, WebSocket, API e Overview. Cabeçalho de identificação + régua de abas numerada + corpo + barra de comandos larga no rodapé substituem o cabeçalho/rodapé particular de cada tela, a coluna vertical "AÇÕES" e os cards de um número só
- Abas Health e Metrics removidas; o sinal delas foi incorporado na faixa de alertas do Overview e nas linhas de status de cada módulo
- Integração com LazyGit (`L` na aba Git) removida
- Wizards de túnel (ngrok, SSH, Cloudflare) mostram o comando CLI exato que vão executar, como preview, antes de confirmar

### Fixed
- Detecção de processos `cloudflared` não reconhecia binário de release baixado sem renomear (`cloudflared-linux-amd64`)
- Túneis ngrok/SSH/Cloudflare reiniciados perdiam domain/region e trocavam de URL; os argumentos agora carregam essa configuração entre reinícios
- `joinWithSpacer` (cabeçalho e régua de várias telas) podia desenhar uma linha mais larga que o terminal quando nome de projeto comprido e vários chips de status não cabiam juntos; agora encolhe o lado esquerdo em vez de estourar a largura

## [1.6.3] - 2026-08-25

### Added
- **Git commit graph**: filtro de branch (`B`, digite pra filtrar a lista), foco alternável entre painéis (commits/detalhe/arquivos) via `tab` com scroll vertical e horizontal independentes, e detalhe de commit mais completo (refs, todos os parents, mensagem completa)

### Changed
- Linhas de imagem Docker (aba Containers) passam a distinguir 3 estados — pertence a este projeto / outro projeto / dangling — em vez de um indicador binário

## [1.6.2] - 2026-08-25

### Added
- **Gerenciamento de imagens Docker** (`i` na aba Containers)
  - Escopo em 3 níveis (`A` alterna): imagem do container selecionado → imagens do projeto (inclui builds antigos sem tag, via label de compose) → todas as imagens do host
  - Indicador visual de quais imagens pertencem ao projeto atual
  - `D` abre modal de remoção com 4 opções (remover / remover só sem tags / forçado / forçado só sem tags) e cancelar
- **Árvore de dependências do Compose** (`Ctrl+G` na aba Containers) — lê `depends_on` do compose (lista ou mapa com `condition`) e desenha em árvore com o status real (running/stopped/não criado) de cada serviço
- **Git commit graph** (`Ctrl+G` na aba Git)
  - Layout de lanes calculado internamente a partir do histórico de commits (não reaproveita o `--graph` do git), desenhando curvas arredondadas (`╭╮╰╯`) e junções (`├┤┴`) em vez de diagonais
  - Cor por lane, ícone por tipo de commit, badges de branch/tag inline na linha do commit
  - Painéis de detalhe do commit (hash, autor, data, parent, mensagem) e arquivos alterados (+/− por arquivo), carregados sob demanda ao navegar
  - `Enter` sobre um commit abre a tela de detalhe completa (árvore de arquivos + diff) já existente na aba Git
- **Resolução de conflitos de Git** na aba Git
  - `p` (pull) detecta divergência e oferece modal **Merge** (`--no-ff`) ou **Rebase**, além de cancelar
  - Modo de conflito (após pull-merge, pull-rebase, `M` ou cherry-pick): lista arquivos em conflito com diff colorido ours (`o`, −) vs theirs (`t`, +)
  - Resolver por arquivo com `o` (ours) / `t` (theirs) / `b` (manter ambos os lados) ou editar manualmente
  - `c` continue / `x` abort da operação em andamento (merge/rebase/cherry-pick)
- **Aba Relax (`Ctrl+T`)** — animações de terminal para descansar a cabeça; nada roda de fato aqui
  - Novas cenas: **Tetris**, **Sword**, **Hourglass**, **Chess**, **Jackpot** e **V4** (motor a pistões)
  - Motor de renderização em Braille compartilhado entre as cenas
  - Cenas antigas (Aquarium, Clouds, Raindrop, Mountains, Invaders) removidas e substituídas pelas novas
- **Compor mensagem WebSocket** (`m`) — modal dedicado com editor, seleção de tipo de frame por tab (`Tab`/`Shift+Tab`) e envio
- **Modo HTTP/2 para Cloudflare Tunnel** — força o transporte em redes que bloqueiam QUIC/UDP 7844; detectado automaticamente na descoberta de túneis do sistema
- **Barras de progresso em Braille** — timeline de jobs e heatmap de falhas da aba Actions passam a usar o mesmo estilo pontilhado do `docker pull`
- **Aba Swarm (MANAGER)** — Control Center de Docker Swarm
  - Visões: Services, Nodes, Tasks, Stacks, Networks, Secrets, Configs e Events
  - Scale, update de imagem, create service, force update e rollback
  - Deploy de stack a partir do compose do projeto
  - Swarm init, join tokens (worker/manager), promote/demote e availability
  - Logs de service, inspect e remoção com confirmação; prune de networks
  - Refresh automático (~5s) e modais de confirmação para ações destrutivas
- **Aba Actions (AUTOMATION)** — GitHub Actions via `gh`
  - Control Center com Processes, Runs e Workflows
  - Catálogo de processos por projeto (`.devscope/actions.yaml`) e templates (ci/deploy/manual)
  - Detalhe do processo: Overview, Runs, Timeline (jobs/steps), Logs e YAML
  - Trigger (`workflow_dispatch`) com branch, inputs e aviso de commits ahead
  - Re-run, cancel/stop (incl. bulk), login `gh` na landing e billing de Actions
  - Notas por run (`.devscope/actions-notes.yaml`) e links para o GitHub
- **Aba SSH Tunnel (TUNNEL)**
  - Cliente `sshutil`: start/stop de túneis locais (`-L`) e remotos (`-R`)
  - Config por projeto em `.devscope/ssh.json`, merge com processos vivos
  - Wizard de criação (nome, modo, porta, bind, target, identity)
  - Overview, Tunnels, History e Settings; seed de porta/target pelo projeto/git
  - Confirmação de exclusão e badges de status (online/offline/starting)
- **Aba Containers — visão de Portas** (`enter`)
  - Lista portas publicadas (host→container, proto tcp/udp, IP de bind)
  - Preview HTTP da porta selecionada (probe assíncrono)
  - "Fechar porta": recria o container sem publicar a porta, com modal de confirmação
- **Containers órfãos e serviços missing**
  - Containers do `docker ps -a` sem projeto escaneado aparecem como órfãos na visão "TODOS + ÓRFÃOS" (`A`)
  - Reclaim automático quando o nome bate com um service do compose do projeto
  - Serviços do compose sem container listados como "missing"; toggle `v` (só docker / c/ missing)
  - Header com contadores running/stopped/missing/orphan
- **Sistema de animações** (`anim.go`)
  - Spinner braille (~10fps) nos estados de loading de todas as abas
  - Pulse animado no indicador de saúde da sidebar
  - Tick de animação só roda enquanto há algo animando (sem re-render em idle)
- **Landing probe assíncrono** (`landing_probe.go`) — disponibilidade de Ngrok, CF Tunnel, SSH, Swarm, Kubernetes, Jenkins e GH Actions verificada em background
- **Banner ASCII** nos scripts de instalação (`install.sh` / `install.ps1`)

### Changed
- **Sidebar reorganizada em 6 grupos**, cada um com cor própria:
  - WATCH (Overview, Metrics, Status) · SCOPE (Git, Containers) · AUTOMATION (Actions, Jenkins) · MANAGER (Swarm, Kubernetes) · TUNNEL (Ngrok, SSH, CF Tunnel) · TOOLS (Rotas, API, Database, WebSocket)
- Help screen com atalhos de Swarm e Actions
- Modais de confirmação genéricos reutilizados (túneis, Swarm e fechar porta)
- Projeto abre direto na aba Git ao dar `enter` no dashboard (antes: Overview)
- Landings de Jenkins, K8s, Ngrok, SSH e CF Tunnel passam a ler apenas campos em cache — a View nunca executa CLI/rede
- `DetectProjectDatabases` dividido: versão Lite sem `docker exec` (segura no render) + enriquecimento assíncrono
- Parse de portas Docker completo (IP de bind, host→container, tcp/udp)
- Rodapé de ações da aba Containers com altura dinâmica (não corta a lista)
- README reescrito e organizado por grupos de módulos

### Fixed
- **Corrupção visual da TUI** (painéis duplicados/empilhados ao navegar) — causada por logs de erro dos coletores em background (`docker ps`, scanner) vazando direto pro stdout/stderr enquanto o Bubble Tea controlava a tela; logs agora vão para `~/.config/devscope/devscope.log`
- Atualizado o Bubble Tea (e dependências) para uma versão com correções de renderer relacionadas a duplicação de linha entre terminais

## [1.4.0] - 2026-08-01

### Added
- **Compose presets** para imagens comuns (Postgres, MySQL, MariaDB, Mongo, Redis, Nginx, Traefik, RabbitMQ, Elasticsearch, Node e outras)
  - Geração automática de portas, env, volumes e healthcheck
  - Fallback por manifesto da imagem ou template mínimo
- **Registry inspect** — lê config pública do Docker Hub (ports/env/volumes) para semear o YAML do compose
- **Docker Hub avançado**
  - Detalhes do repositório, listagem de tags e paginação na busca
  - Formatação de downloads, tamanho e datas relativas
- **Wizard Add Container** em 3 passos (Busca → Imagem → Compose)
  - Painéis de resultados, tags e detalhes; edição manual do YAML
  - Render dedicado (`docker_add_render`)
- **Modais compartilhados de túnel** (Cloudflare e Ngrok)
  - Confirmação de exclusão, badges de status e layout unificado
- **Aba Git redesenhada**
  - Coluna lateral: activity, stashes e remotes
  - Log de comandos com URLs clicáveis (mouse)
  - Árvore de arquivos no working tree e no diff do commit
  - Filtro de branches ao vivo (`b`)
- **Restart policy de containers** — exibição na lista e alteração via UI (`docker update --restart`)
- **Temas novos e refinados**
  - `devscope` (padrão), `tokyo-night`, `rose-pine`, `dracula-vivid`
  - Aliases (`github`, `tokyonight`, `mocha`, …) e paletas ajustadas para TUI
  - Tema padrão no exemplo de config: `devscope`

### Changed
- Aba Cloudflare e Ngrok: painel de detalhes, wizard e confirmações aprimorados
- Merge de compose passa a unir volumes de top-level ao adicionar serviços
- Collectors Git com helpers de exec (`GitExec`, checkout/pull/push com output)
- Dashboard e app: suporte a mouse e atalhos extras na aba Git/Containers

### Fixed
- Layout de colunas na lista de containers para não quebrar linha no terminal

## [1.3.1] - 2026-07-29

### Added
- **Aba Cloudflare Tunnel (TOOLS)**
  - Cliente `cfutil`: agent, config e descoberta de túneis
  - Listagem, criação (wizard), detalhes e exclusão de túneis por projeto
  - Integração na sidebar e no fluxo de módulos

### Changed
- Polimento de layout em várias abas (module shell, sidebar, overview, ngrok, k8s, ws)

## [1.3.0] - 2026-07-23

### Added
- **Configuração de projeto para WebSocket URLs**
  - Gerenciamento de URLs de WebSocket por projeto (salvar/carregar de JSON)
  - Limpeza e deduplicação de URLs
- **Aba Jenkins (CI/CD)**
  - Cliente Jenkins completo com listagem de jobs, builds, console output
  - Gerenciamento de nós, plugins e fila de builds
- **Container Detail Views**
  - Visualização detalhada de containers com abas de info, env, ports, mounts, logs, stats
  - Monitoramento em tempo real com ContainerStats
- **Docker Compose Edit**
  - Editor de docker-compose.yml embutido
  - Adicionar/remover serviços, portas, volumes, env vars
- **Docker Hub Integration**
  - Busca e pull de imagens do Docker Hub
- **Git tab aprimorada**
  - Integração com docker-compose (up/down/restart via git tab)
  - Mensagens de commit e prompt aprimorados
- **Add Container Dialog**
  - Wizard para adicionar containers ao projeto (search + config)
- **Aprimoramentos cross-platform**
  - ProjectShell unificado para Windows e Unix
  - Execução de comandos shell adaptada ao SO
- **Melhorias em abas existentes**
  - Database tab: descoberta de schemas aprimorada, queries nomeadas
  - JSON tab: validação e busca aprimoradas
  - JWT tab: geração e verificação de tokens
  - Routes tab: suporte a Laravel e NestJS aprimorado
  - WebSocket tab: gerenciamento de conexões, filtros e mensagens
  - Tema: sistema de temas completo com tema escuro/claro
- Testes unitários para WebSocket config, Jenkins, Container stats, Docker compose, temas

## [1.2.0] - 2026-07-23

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

[Unreleased]: https://github.com/PirataZang/devscope/compare/v1.6.3...HEAD
[1.6.3]: https://github.com/PirataZang/devscope/compare/v1.6.2...v1.6.3
[1.6.2]: https://github.com/PirataZang/devscope/compare/v1.4.0...v1.6.2
[1.4.0]: https://github.com/PirataZang/devscope/compare/v1.3.1...v1.4.0
[1.3.1]: https://github.com/PirataZang/devscope/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/PirataZang/devscope/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/PirataZang/devscope/compare/v1.0.0...v1.2.0
[1.0.0]: https://github.com/PirataZang/devscope/compare/v0.1.2...v1.0.0
[0.1.2]: https://github.com/PirataZang/devscope/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/PirataZang/devscope/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/PirataZang/devscope/releases/tag/v0.1.0
