# DevScope

<p align="center">
  <strong>htop dos projetos</strong> — visualize e opere todos os projetos da sua VPS em uma única TUI.
</p>

<p align="center">
  <a href="https://github.com/devscope/devscope/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/devscope/devscope/ci.yml?branch=main&label=CI" alt="CI"></a>
  <a href="https://github.com/devscope/devscope/releases"><img src="https://img.shields.io/github/v/release/devscope/devscope?label=release" alt="Release"></a>
  <a href="https://github.com/devscope/devscope/blob/main/LICENSE"><img src="img.shields.io/badge/license-MIT-blue.svg" alt="License MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.22+-00ADD8?logo=go&logoColor=white" alt="Go 1.22+"></a>
</p>

<!-- Demo GIF: adicionar em v1.0 -->

---

## O problema

Em uma VPS Linux típica, entender o estado de tudo que roda exige dezenas de comandos:

```bash
docker ps -a
docker stats --no-stream
pm2 list
git -C /var/www/projeto status
ss -ltn | grep LISTEN
curl -sf localhost:3000/health
certbot certificates
nginx -T | grep server_name
```

Cada ferramenta expõe uma **unidade diferente** (container, processo, serviço). Você pensa em **projetos**.

## A solução

```bash
devscope
```

Um binário. Zero configuração obrigatória. Scan automático. Tudo agrupado por projeto.

```
┌───────────────────────────────────────────────────────────────────────┐
│ DevScope v0.1.0          CPU 21%   RAM 54%   DISK 31%        14:32:01 │
├───────────────────────────────────────────────────────────────────────┤
│ SYSTEM OVERVIEW                                                       │
│ Uptime: 12d 4h  •  Load: 0.42  •  Docker: 8  •  RAM: 8192/16384 MB    │
├───────────────────────────────────────────────────────────────────────┤
│ PROJECTS (12)                                                         │
│   NAME              STATUS    BRANCH   CPU   RAM    PORTS             │
│ ● projeto           ● Run     main     12%   340M   :3000             │
│ ● projeto           ● Deg     develop   8%   128M   :5173             │
│ ○ projeto           ● Stop    main      -     -     -                 │
├───────────────────────────────────────────────────────────────────────┤
│ Total: 12   Running: 8   Stopped: 3   Degraded: 1                     │
│ ↑↓ navigate  ENTER open  / filter  g git  c containers  ? help  q quit│
└───────────────────────────────────────────────────────────────────────┘
```

Painel de projeto:

```
┌──────────────────────────────────────────────────────────────────────┐
│ projeto  NestJS  ● Running  │  Health: Healthy                       │
├──────────────────────────────────────────────────────────────────────┤
│ Overview │ Git │ Containers │ Health │ Logs │ Metrics                │
├──────────────────────────────────────────────────────────────────────┤
│ Path:      /var/www/projeto                                          │
│ Framework: NestJS (TypeScript)                                       │
│ Workers:   2 PM2 (api, worker)                                       │
│ Domains:   app.example.com → :3000  (SSL: 45d)                       │
│ Git:       main  •  2 modified  •  ahead 1                           │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Por que DevScope?

| Ferramenta | Unidade | DevScope |
|------------|---------|----------|
| [LazyDocker](https://github.com/jesseduffield/lazydocker) | Container | Agrupa containers **por projeto** |
| [LazyGit](https://github.com/jesseduffield/lazygit) | Repositório | Git integrado no contexto do projeto |
| `docker ps` | Container | Status, métricas, health, domínios |
| `htop` | Processo | Visão orientada a **projeto** |
| Portainer | Web UI | TUI nativa, binário único, sem browser |

DevScope **complementa** LazyDocker e LazyGit — abre lazygit (`L`) no diretório do projeto sem sair da TUI.

---

## Features

### Disponível agora

- **Descoberta automática** — scan de `/var/www`, `/home`, `/opt` e paths customizados
- **Detectores de framework** — NestJS, Laravel, Django, Next.js, Vue, React, Nuxt, Go, Python, Rust, PHP, Java
- **Dashboard TUI** — lista de projetos com status, branch, CPU/RAM, portas
- **Métricas de host** — CPU, RAM, Disk, Swap, Load, Uptime via `/proc`
- **Docker** — `docker ps`, stats, inspect, correlação container→projeto
- **Git** — branch, commits, working tree, diff de arquivos (read-only)
- **PM2** — workers por projeto via `pm2 jlist`
- **Health checks** — HTTP/TCP com status `Degraded`
- **Portas** — detecção via containers e `/proc/net/tcp`
- **Nginx + SSL** — domínios e dias restantes do certificado Let's Encrypt
- **Ações** — shell no projeto/container, restart, pause, remove, logs (follow)
- **Deploy** — detecta e executa scripts de deploy com confirmação
- **Temas** — dark / light / auto
- **CLI** — `devscope scan --json`, `devscope watch`
- **Instalação** — `curl | bash`, `go install`, GitHub Releases

### Em desenvolvimento

- Multi-host via SSH (v2.0)
- Alertas e webhooks (Slack, Discord)
- Integração CI (GitHub Actions / GitLab)
- GIF demo no README

---

## Instalação

### Instalação rápida (curl)

Linux amd64/arm64:

```bash
curl -fsSL https://raw.githubusercontent.com/devscope/devscope/main/scripts/install.sh | bash
```

Versão específica ou diretório customizado:

```bash
DEVSCOPE_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/devscope/devscope/main/scripts/install.sh | bash

DEVSCOPE_INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/devscope/devscope/main/scripts/install.sh | bash
```

### Go install

Requer [Go 1.22+](https://go.dev/dl/):

```bash
go install github.com/devscope/devscope/cmd/devscope@latest
```

Certifique-se de que `$GOPATH/bin` ou `$HOME/go/bin` está no `PATH`.

### Build from source

```bash
git clone https://github.com/devscope/devscope.git
cd devscope
make run          # build + roda ./bin/devscope (desenvolvimento)
make install-dev  # atualiza o `devscope` do PATH (ex.: /usr/local/bin)
make build        # só compila em bin/devscope
```

### Release binary

Baixe em [GitHub Releases](https://github.com/devscope/devscope/releases) — inclui `checksums.txt` para verificação.

---

## Quick Start

```bash
# 1. Instalar
curl -fsSL https://raw.githubusercontent.com/devscope/devscope/main/scripts/install.sh | bash

# 2. Abrir (scan automático, sem config obrigatória)
devscope

# 3. (Opcional) Config customizada
mkdir -p ~/.config/devscope
cp configs/devscope.example.yaml ~/.config/devscope/config.yaml
devscope --config ~/.config/devscope/config.yaml
```

Modos sem TUI:

```bash
devscope scan --json          # snapshot JSON para scripts/Ansible
devscope watch                # refresh no terminal
devscope version
```

---

## Atalhos

### Dashboard

| Tecla | Ação |
|-------|------|
| `↑` / `↓` / `k` / `j` | Navegar projetos |
| `Enter` | Abrir projeto |
| `/` | Filtrar projetos |
| `Ctrl+P` | Busca fuzzy global |
| `g` | Abrir tab Git |
| `c` | Abrir tab Containers |
| `Shift+E` | Terminal no diretório do projeto |
| `r` | Atualizar snapshot |
| `?` | Ajuda |
| `q` | Sair |

### Projeto

| Tecla | Ação |
|-------|------|
| `Esc` | Voltar ao dashboard |
| `Tab` / `Shift+Tab` | Próxima / anterior tab |
| `1`–`6` | Overview, Git, Containers, Health, Logs, Metrics |
| `L` | Abrir LazyGit no projeto |
| `D` | Deploy (com confirmação) |
| `o` | Abrir URL no browser |
| `h` | Tab Health |
| `l` | Tab Logs |

### Git

| Tecla | Ação |
|-------|------|
| `←` / `→` / `h` / `l` | Alternar seções |
| `b` | Filtrar branches |
| `Enter` | Ver commits / arquivos (read-only) |

### Containers

| Tecla | Ação |
|-------|------|
| `Shift+E` | Shell no container |
| `p` | Pause |
| `r` | Restart |
| `d` | Remove (confirma com `y`) |
| `m` | Logs |
| `f` | Follow logs (pause com `p`) |

---

## Configuração

Copie o exemplo:

```bash
mkdir -p ~/.config/devscope
cp configs/devscope.example.yaml ~/.config/devscope/config.yaml
```

Referência completa ([configs/devscope.example.yaml](configs/devscope.example.yaml)):

```yaml
scan:
  paths:
    - /var/www
    - /home
    - /opt
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
  theme: auto          # dark | light | auto

health:
  timeout: 5s
  concurrent: 10

pinned:
  - /var/www/projeto
```

### Variáveis de ambiente

Prefixo `DEVSCOPE_` — ex: `DEVSCOPE_SCAN_PATHS=/var/www,/home/usuario/projetos`

---

## Arquitetura

```
ScanPaths → Walk filesystem → Detectores → Enrichment
         → Collectors (Docker, PM2, Git, Health, Nginx, SSL)
         → Snapshot imutável → Bubble Tea UI
```

Detalhes em [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

**Stack:** [Go](https://go.dev/) · [Bubble Tea](https://github.com/charmbracelet/bubbletea) · [Lip Gloss](https://github.com/charmbracelet/lipgloss) · [Cobra](https://github.com/spf13/cobra) · [Viper](https://github.com/spf13/viper)

---

## Roadmap

| Versão | Foco |
|--------|------|
| **v0.1** | Docker stats, PM2, Degraded, portas, release pública |
| **v0.2** | Health, logs follow, nginx, SSL |
| **v0.3** | Temas, deploy, LazyGit, fuzzy finder |
| **v1.0** | `scan --json`, `watch`, testes integração, GIF demo |
| **v2.0** | Multi-host SSH, alertas, webhooks |

Detalhes em [docs/ROADMAP.md](docs/ROADMAP.md).

---

## Contribuindo

Contribuições são bem-vindas! Veja [CONTRIBUTING.md](CONTRIBUTING.md).

- Bug? [Abra uma issue](https://github.com/devscope/devscope/issues/new?template=bug_report.md)
- Feature? [Solicite aqui](https://github.com/devscope/devscope/issues/new?template=feature_request.md)
- Novo framework? [Detector request](https://github.com/devscope/devscope/issues/new?template=detector_request.md)

---

## Publicar uma release

```bash
git tag v0.1.0
git push origin v0.1.0
```

O GitHub Actions (GoReleaser) gera binários linux/amd64 e linux/arm64 + `checksums.txt`.

---

## Licença

[MIT](LICENSE)

---

## Demo (em breve)

> GIF de demonstração será adicionado na v1.0 — grave a TUI com [vhs](https://github.com/charmbracelet/vhs) ou asciinema e substitua este bloco.
