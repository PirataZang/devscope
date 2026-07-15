# DevScope — Arquitetura

> **htop dos projetos** — uma TUI que agrupa containers, serviços, git, deploy, health e métricas sob a abstração de **Projeto**.

---

## 1. Visão Geral

### 1.1 Problema

Em uma VPS Linux típica, um desenvolvedor precisa executar dezenas de comandos (`docker ps`, `pm2 list`, `systemctl`, `git status`, `journalctl`, `nginx -t`, `ss -ltn`, etc.) para entender o estado de todos os sistemas rodando. Cada ferramenta expõe uma **unidade diferente** (processo, container, serviço), mas o desenvolvedor pensa em **projetos**.

### 1.2 Solução

O DevScope escaneia o filesystem, detecta projetos automaticamente, agrupa serviços relacionados e apresenta tudo em uma TUI fluida inspirada em LazyDocker, LazyGit, k9s e htop.

### 1.3 Princípios de Design

| Princípio | Descrição |
|-----------|-----------|
| **Projeto como unidade** | Tudo gira em torno de `Project`, não de processos ou containers isolados |
| **Descoberta automática** | Zero configuração obrigatória; scan de diretórios padrão |
| **Binário único** | Sem Node, Python, Java ou Docker para executar |
| **UI nunca bloqueia** | Toda coleta é concorrente; UI consome snapshots imutáveis |
| **Plugins extensíveis** | Cada framework tem seu detector; novos frameworks = novo plugin |
| **Graceful degradation** | Se Docker não está instalado, o resto funciona normalmente |

---

## 2. Decisões Arquiteturais

### 2.1 Por que Go + Bubble Tea?

- **Go**: compilação estática, cross-compile nativo (amd64/arm64), excelente concorrência, subprocess/exec robusto para integrar com ferramentas do sistema
- **Bubble Tea**: modelo Elm (Model-Update-View), TUI performática, ecossistema Charm (Bubbles, Lip Gloss)
- **Cobra + Viper**: CLI padrão da indústria Go; config em YAML/TOML/env

### 2.2 Modelo de Dados Imutável (Snapshot)

A UI **nunca** lê dados mutáveis diretamente. Um `StateStore` mantém snapshots atômicos:

```
Collector (goroutine) → muta dados internos com mutex
                      → publica ProjectSnapshot (cópia imutável)
UI (Bubble Tea tick)  → lê snapshot via atomic.Value ou RWMutex
```

Isso elimina race conditions entre coletores e renderização.

### 2.3 Pipeline de Descoberta

```
ScanPaths → Walk filesystem → Marker detection → Framework plugins
         → Service detection → Project grouping → Enrichment → Snapshot
```

Cada etapa é independente e pode falhar sem derrubar as demais.

### 2.4 Agrupamento de Projetos

Um diretório raiz com marcadores (`package.json`, `docker-compose.yml`, etc.) define um **projeto**. Subdiretórios com roles conhecidos são agrupados:

```
projeto-api/
├── backend/     → role: backend
├── frontend/    → role: frontend
├── worker/      → role: worker
├── docker-compose.yml
└── .git/
```

Heurísticas de agrupamento:
1. `docker-compose.yml` na raiz → todos os services do compose pertencem ao projeto
2. Monorepo com workspaces (`package.json` workspaces, `go.work`) → um projeto, múltiplos módulos
3. Subpastas `frontend/`, `backend/`, `api/`, `worker/`, `cron/` → roles automáticos
4. PM2 ecosystem file → apps agrupadas pelo `cwd` comum

### 2.5 Detecção de Framework (Plugin System)

Cada plugin implementa:

```go
type Detector interface {
    Name() string
    Priority() int          // maior = verificado primeiro
    Detect(ctx context.Context, root string) (*FrameworkInfo, error)
}
```

Detecção por **marcadores de arquivo** + **conteúdo** (ex: `dependencies` em `package.json` contendo `@nestjs/core`).

Ordem de prioridade: frameworks específicos (NestJS, Laravel) antes de genéricos (Node, PHP).

### 2.6 Detecção de Serviços

Serviços são detectados em duas fases:

1. **Estática** (no scan): presença de `docker-compose.yml`, `ecosystem.config.js`, `Procfile`, configs nginx
2. **Dinâmica** (polling): `docker ps`, `pm2 jlist`, `systemctl list-units`, portas via `/proc/net/tcp`

Correlação serviço→projeto:
- Docker: label `com.devscope.project` (futuro) ou match por `working_dir` / volume mount path
- PM2: `pm_cwd` dentro do path do projeto
- systemd: `WorkingDirectory=` no unit file
- Portas: processo escutando em porta declarada no compose/nginx

### 2.7 Health Checks

```go
type HealthChecker interface {
    Check(ctx context.Context, target HealthTarget) HealthResult
}
```

Tipos: HTTP GET, HTTPS + SSL validation, TCP connect, ICMP ping (se permitido).

Health targets derivados de: `docker-compose` healthcheck, nginx `proxy_pass`, PM2 env `PORT`, `.env` files.

### 2.8 Logs Unificados

Abstração `LogSource`:

```go
type LogSource interface {
    Stream(ctx context.Context, opts LogOptions) (<-chan LogLine, error)
}
```

Implementações: `DockerLogSource`, `PM2LogSource`, `SystemdLogSource`, `FileLogSource`.

A UI aplica filtro/busca/regex no client-side sobre o buffer circular (ring buffer de 10k linhas).

---

## 3. Estrutura de Pastas

```
devscope/
├── cmd/
│   └── devscope/
│       └── main.go                 # Entry point mínimo
├── internal/
│   ├── app/
│   │   └── app.go                  # Bootstrap: config → store → collectors → TUI
│   ├── config/
│   │   └── config.go               # Viper: paths, intervals, toggles
│   ├── core/
│   │   ├── models.go               # Project, Service, Container, GitInfo, etc.
│   │   ├── state.go                # StateStore com snapshots atômicos
│   │   └── events.go               # Event bus interno (opcional v0.2+)
│   ├── scanner/
│   │   ├── scanner.go              # Walk filesystem, encontra projetos
│   │   ├── markers.go              # Marcadores de identificação
│   │   └── grouper.go              # Agrupa subdirs em roles
│   ├── detectors/
│   │   ├── registry.go             # Registro e ordenação de plugins
│   │   ├── node.go                 # package.json genérico
│   │   ├── nestjs.go
│   │   ├── laravel.go
│   │   ├── django.go
│   │   ├── go.go
│   │   ├── rust.go
│   │   ├── python.go
│   │   ├── react.go
│   │   ├── vue.go
│   │   ├── next.go
│   │   ├── nuxt.go
│   │   └── docker.go               # Projeto só-docker (sem código)
│   ├── collectors/
│   │   ├── manager.go              # Orquestra todos os collectors
│   │   ├── docker.go
│   │   ├── pm2.go
│   │   ├── systemd.go
│   │   ├── git.go
│   │   ├── nginx.go
│   │   ├── ports.go
│   │   └── ssl.go
│   ├── health/
│   │   ├── checker.go
│   │   ├── http.go
│   │   ├── tcp.go
│   │   └── ssl.go
│   ├── metrics/
│   │   ├── host.go                 # CPU, RAM, Disk, Swap via /proc
│   │   └── process.go              # Métricas por projeto/processo
│   ├── logs/
│   │   ├── source.go               # Interface LogSource
│   │   ├── docker.go
│   │   ├── pm2.go
│   │   ├── systemd.go
│   │   └── file.go
│   ├── commands/
│   │   └── actions.go              # Restart, Shell, Deploy shortcuts
│   └── ui/
│       ├── app.go                  # Bubble Tea root model
│       ├── styles.go               # Lip Gloss theme
│       ├── keys.go                 # Keybindings globais
│       ├── dashboard/
│       │   └── dashboard.go        # Lista de projetos
│       ├── project/
│       │   ├── detail.go           # Painel do projeto (tabs)
│       │   ├── containers.go
│       │   ├── logs.go
│       │   ├── git.go
│       │   └── metrics.go
│       └── components/
│           ├── table.go
│           ├── statusbar.go
│           ├── header.go
│           └── spinner.go
├── pkg/
│   └── version/
│       └── version.go              # ldflags injection
├── docs/
│   ├── ARCHITECTURE.md
│   └── ROADMAP.md
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── configs/
│   └── devscope.example.yaml
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## 4. Descrição dos Pacotes

### 4.1 `cmd/devscope`

Entry point. Chama `internal/app.Run()`. Sem lógica de negócio.

### 4.2 `internal/app`

Bootstrap da aplicação:
1. Parse flags (Cobra)
2. Load config (Viper)
3. Criar `StateStore`
4. Iniciar `CollectorManager` (goroutines)
5. Iniciar Bubble Tea program
6. Graceful shutdown via `context.Context`

### 4.3 `internal/config`

```yaml
scan:
  paths:
    - /var/www
    - /home
    - /opt
    - /srv
    - /workspace
  max_depth: 5
  ignore:
    - node_modules
    - vendor
    - .git

refresh:
  scan_interval: 60s
  metrics_interval: 2s
  health_interval: 10s
  git_interval: 30s

ui:
  theme: auto  # auto | dark | light

health:
  timeout: 5s
  concurrent: 10
```

Precedência: flags > env (`DEVSCOPE_*`) > config file > defaults.

### 4.4 `internal/core`

**models.go** — tipos centrais:

```go
type Project struct {
    ID          string
    Name        string
    Path        string
    Framework   FrameworkInfo
    Status      ProjectStatus  // Running, Stopped, Degraded, Unknown
    Health      HealthStatus   // Healthy, Unhealthy, Unknown
    Git         *GitInfo
    Services    []Service
    Containers  []Container
    Workers     []Worker
    Domains     []Domain
    SSL         []SSLCert
    Metrics     ProjectMetrics
    Ports       []int
    LastDeploy  *time.Time
    Uptime      time.Duration
    Modules     []ProjectModule  // subdirs agrupados
}

type FrameworkInfo struct {
    Name    string  // "NestJS", "Laravel", "Go"
    Version string
    Language string
}

type Service struct {
    Type     ServiceType  // Docker, PM2, Systemd, Nginx, Redis, etc.
    Name     string
    Status   string
    PID      int
    Port     int
    CPU      float64
    Memory   int64
    Role     string  // frontend, backend, worker, database
}
```

**state.go** — `StateStore`:

```go
type StateStore struct {
    mu       sync.RWMutex
    snapshot *Snapshot
}

type Snapshot struct {
    Projects    []Project
    HostMetrics HostMetrics
    ScannedAt   time.Time
    ScanPaths   []string
}

func (s *StateStore) Get() *Snapshot       // UI chama isso
func (s *StateStore) Update(fn func(*Snapshot)) // Collectors chamam isso
```

Usa **copy-on-write**: `Update` clona o snapshot, aplica mudanças, troca atomicamente.

### 4.5 `internal/scanner`

Responsável pela descoberta inicial e rescans periódicos.

**Fluxo:**
1. Para cada path configurado, `filepath.WalkDir` com `max_depth`
2. Em cada diretório, verifica `markers.go` — lista de arquivos indicadores
3. Se encontrou marcador, cria candidato a `Project`
4. `grouper.go` analisa subdirs e associa roles
5. Passa para `detectors/registry.go` para identificar framework
6. Retorna `[]Project` (sem dados dinâmicos — isso vem dos collectors)

**Otimizações:**
- Cache de diretórios já escaneados (mtime check)
- Skip de `node_modules`, `vendor`, `.git` internals
- Worker pool limitado (ex: 4 goroutines de walk paralelo)

### 4.6 `internal/detectors`

Sistema de plugins. Cada detector é um arquivo separado.

**Registry:**

```go
var registry []Detector

func Register(d Detector) { registry = append(registry, d) }
func DetectAll(ctx context.Context, root string) FrameworkInfo {
    sort.Slice(registry, func(i, j int) bool {
        return registry[i].Priority() > registry[j].Priority()
    })
    for _, d := range registry {
        if info, err := d.Detect(ctx, root); err == nil && info != nil {
            return *info
        }
    }
    return FrameworkInfo{Name: "Unknown"}
}
```

**Exemplo NestJS detector:**
1. Existe `package.json`?
2. Parse JSON → `dependencies` contém `@nestjs/core`?
3. Return `FrameworkInfo{Name: "NestJS", Language: "TypeScript"}`

### 4.7 `internal/collectors`

Cada collector roda em sua própria goroutine com ticker configurável.

**Manager:**

```go
type Manager struct {
    store   *StateStore
    cfg     *config.Config
    cancel  context.CancelFunc
}

func (m *Manager) Start(ctx context.Context) {
    go m.runScanner(ctx)      // 60s
    go m.runDocker(ctx)       // 5s
    go m.runPM2(ctx)          // 5s
    go m.runSystemd(ctx)      // 10s
    go m.runGit(ctx)          // 30s
    go m.runMetrics(ctx)      // 2s
    go m.runHealth(ctx)       // 10s
    go m.runNginx(ctx)        // 30s
    go m.runSSL(ctx)          // 60s
    go m.runPorts(ctx)        // 5s
}
```

Cada collector:
1. Coleta dados do sistema (exec, HTTP, /proc)
2. Correlaciona com projetos existentes no snapshot
3. Chama `store.Update()` com função que modifica apenas seu domínio

**Isolamento de falhas:** cada collector tem `recover()` e loga erro sem parar os demais.

### 4.8 `internal/health`

Executa checks concorrentes com semáforo (`concurrent` config).

Resultado cacheado por `health_interval`. UI mostra último resultado + timestamp.

### 4.9 `internal/metrics`

**Host metrics** via `/proc/stat`, `/proc/meminfo`, `/proc/diskstats`, `syscall.Statfs`.

**Process metrics** via `/proc/[pid]/stat` e `/proc/[pid]/status`.

Correlação: soma métricas de todos os PIDs/containers do projeto.

### 4.10 `internal/logs`

Ring buffer por fonte de log. Follow mode usa `exec.Command` com pipe + goroutine de leitura.

Filtro/regex aplicado na UI para não reprocessar no backend.

### 4.11 `internal/ui`

Bubble Tea com modelo hierárquico:

```
AppModel
├── View: Dashboard | ProjectDetail | Logs | Help
├── DashboardModel (lista de projetos, table)
└── ProjectDetailModel
    ├── Tab: Overview
    ├── Tab: Containers
    ├── Tab: Logs
    ├── Tab: Git
    ├── Tab: Services
    └── Tab: Metrics
```

**Atualização da UI:**
- `tea.Tick` a cada 1s → lê snapshot do `StateStore` → atualiza model → `View()`
- Keybindings globais em `keys.go`
- Lip Gloss para estilização consistente

**Keybindings:**

| Tecla | Ação |
|-------|------|
| `R` | Restart serviço selecionado |
| `S` | Shell no container/diretório |
| `L` | Abrir logs |
| `G` | Abrir git info |
| `D` | Deploy (se script detectado) |
| `H` | Health check manual |
| `B` | Abrir no browser (xdg-open) |
| `P` | Listar processos |
| `Enter` | Abrir projeto |
| `Esc` | Voltar |
| `Q` | Sair |
| `/` | Busca/filtro |
| `Tab` | Próxima aba |

### 4.12 `internal/commands`

Ações executadas sob demanda (não polling). Disparam `exec.Command` com confirmação na UI.

---

## 5. Fluxo de Execução

```
main()
  └─ cobra.Execute()
       └─ app.Run()
            ├─ config.Load()
            ├─ store := core.NewStateStore()
            ├─ collectors.NewManager(store, cfg).Start(ctx)
            │    ├─ [goroutine] scanner → store.Update(projects)
            │    ├─ [goroutine] docker  → store.Update(containers)
            │    ├─ [goroutine] pm2     → store.Update(workers)
            │    ├─ [goroutine] git     → store.Update(git info)
            │    ├─ [goroutine] metrics → store.Update(host + project metrics)
            │    └─ [goroutine] health  → store.Update(health status)
            └─ ui.NewApp(store, cfg).Run()
                 ├─ Init() → render dashboard
                 ├─ Update(tea.Tick) → store.Get() → refresh view
                 ├─ Update(tea.KeyMsg) → navegação/ações
                 └─ View() → string renderizada
```

**Shutdown:**
- `Q` ou `Ctrl+C` → cancel context → collectors param → TUI encerra

---

## 6. Arquitetura Concorrente

### 6.1 Diagrama

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Scanner    │────▶│              │◀────│  UI (BT)    │
│  Docker     │────▶│  StateStore  │     │  1s tick    │
│  PM2        │────▶│  (snapshot)  │     └─────────────┘
│  Git        │────▶│              │
│  Metrics    │────▶│              │
│  Health     │────▶└──────────────┘
└─────────────┘
   goroutines
   independentes
```

### 6.2 Evitando Race Conditions

1. **Snapshot imutável**: UI só lê; collectors só escrevem via `Update()` com mutex
2. **Copy-on-write**: `Update` clona snapshot antes de modificar
3. **Sem ponteiros compartilhados na UI**: `Get()` retorna cópia ou snapshot read-only
4. **Collectors não se comunicam entre si**: cada um lê o snapshot atual e escreve sua parte
5. **Channels para logs**: stream unidirecional, sem shared state

### 6.3 Worker Pool

Scanner usa pool limitado para walks paralelos:

```go
pool := make(chan struct{}, 4) // max 4 walks simultâneos
```

Health checks usam semáforo configurável (`health.concurrent`).

### 6.4 Context e Cancelamento

Tudo propagado via `context.Context`. Shutdown cancela todos os collectors e log streams.

---

## 7. Gerenciamento de Memória

1. **Ring buffers para logs**: máximo 10.000 linhas por fonte; descarta as mais antigas
2. **Snapshot lean**: não armazenar output completo de logs no snapshot — só metadata
3. **Scanner cache**: mapa `path → mtime`; re-escaneia apenas se mtime mudou
4. **Evitar alocações na UI**: reutilizar `strings.Builder` no `View()`; preallocate slices
5. **JSON parsing**: stream decode quando possível; não manter JSON raw após parse
6. **Profiling**: `pprof` endpoint opcional em modo debug (`--debug`)

---

## 8. Estratégia de Testes

### 8.1 Unitários

- `detectors/*`: tabelas de teste com diretórios fake (`testdata/`)
- `scanner/markers`: verifica detecção de marcadores
- `core/state`: testa copy-on-write e concorrência
- `health/*`: mock HTTP server para checks

### 8.2 Integração

- `testdata/projects/`: projetos fake (nestjs, laravel, django, go)
- Testa scanner + detectors end-to-end
- Docker tests: `//go:build integration` — rodam só com Docker disponível

### 8.3 UI

- Bubble Tea `teatest` package para testar modelos
- Verifica navegação, keybindings, renderização

### 8.4 Benchmarks

- `BenchmarkScanner` com árvore grande de diretórios
- `BenchmarkSnapshotUpdate` com concorrência

---

## 9. CI/CD

### 9.1 CI (`.github/workflows/ci.yml`)

```yaml
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: make test
      - run: make lint

  build-matrix:
    strategy:
      matrix:
        goos: [linux]
        goarch: [amd64, arm64]
    steps:
      - run: make build GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }}
```

### 9.2 Release (`.github/workflows/release.yml`)

Trigger: tag `v*`

- GoReleaser gera binários para linux/amd64 e linux/arm64
- Checksums SHA256
- GitHub Release com changelog automático

### 9.3 Makefile targets

```
build    — compila binário local
test     — go test ./...
lint     — golangci-lint
release  — goreleaser release
install  — go install ./cmd/devscope
```

---

## 10. Distribuição e Versionamento

### 10.1 Versionamento

Semver: `MAJOR.MINOR.PATCH`

- Injetado via ldflags: `pkg/version.Version`
- Comando `devscope version` exibe versão, commit, build date

### 10.2 Distribuição

| Canal | Método |
|-------|--------|
| GitHub Releases | Binários pré-compilados (.tar.gz) |
| Script | `curl -fsSL https://raw.githubusercontent.com/devscope/devscope/main/scripts/install.sh \| bash` |
| `go install` | `go install github.com/devscope/devscope/cmd/devscope@latest` |

### 10.3 Binário

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o devscope
```

Tamanho esperado: ~15-25MB (comprimido ~8MB).

---

## 11. Segurança

- **Sem elevação de privilégio**: roda como usuário atual
- **Read-only por padrão**: ações (restart, deploy) requerem confirmação
- **Sem rede inbound**: apenas health checks outbound
- **Sanitização de paths**: não segue symlinks para fora dos scan paths
- **Sem execução arbitrária**: commands só executam ações pré-definidas

---

## 12. Limitações Conhecidas (MVP)

- Sem suporte Windows/macOS (Linux only)
- Podman tratado como compat Docker (via socket)
- Kafka/RabbitMQ: detecção básica por porta, sem management API
- Deploy: detecta scripts (`deploy.sh`, Makefile target) mas não executa pipelines CI
- SSL: leitura de certs nginx/letsencrypt, sem renovação automática

---

## 13. Referências

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Cobra](https://github.com/spf13/cobra) — CLI framework
- [Viper](https://github.com/spf13/viper) — Configuration
- [LazyDocker](https://github.com/jesseduffield/lazydocker) — Inspiração UX
- [k9s](https://github.com/derailed/k9s) — Inspiração dashboard
