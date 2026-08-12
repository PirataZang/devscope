# DevScope

<p align="center">
  <strong>O "htop" dos seus projetos</strong> — visualize, monitore e opere todos os projetos da sua máquina ou VPS em uma única TUI (Interface de Terminal).
</p>

<p align="center">
  <a href="https://github.com/PirataZang/devscope/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/PirataZang/devscope/ci.yml?branch=main&label=CI&style=flat-square" alt="CI"></a>
  <a href="https://github.com/PirataZang/devscope/releases"><img src="https://img.shields.io/github/v/release/PirataZang/devscope?label=release&style=flat-square" alt="Release"></a>
  <a href="https://github.com/PirataZang/devscope/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg?style=flat-square" alt="License MIT"></a>
  <img src="https://img.shields.io/badge/platforms-Linux%20%7C%20macOS%20%7C%20Windows-informational?style=flat-square" alt="Platforms">
</p>

---

## ⚡ Instalação Rápida

> **Nenhuma dependência necessária** — não precisa de Go, Docker ou qualquer runtime instalado. Apenas execute o comando abaixo e use.

### 🐧 Linux & 🍎 macOS

```bash
curl -fsSL https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.sh | bash
```

### 🪟 Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.ps1 | iex
```

Após a instalação (reinicie o terminal se necessário):

```bash
devscope
```

> 💡 Instalar uma versão específica, mudar o diretório de instalação ou fazer build from source? Veja [Instalação Avançada](#️-instalação-avançada).

---

## 🚀 Como Usar (Quick Start)

Após instalar, basta rodar:

```bash
devscope
```

O DevScope faz um scan automático nos diretórios mais comuns (`/var/www`, `/home`, `/opt`, …) e abre o painel interativo.

### 🔧 Outros Modos de Uso

```bash
devscope scan --json          # Snapshot completo do servidor em JSON (útil para automações)
devscope watch                # Painel com auto-refresh no terminal
devscope version              # Informações de versão e build
```

---

## 🔍 O Problema

Gerenciar vários projetos ao mesmo tempo exige saltar entre utilitários isolados:

```bash
docker ps -a                    # Ver contêineres rodando
docker stats --no-stream        # Monitorar uso de recursos
pm2 list                        # Processos Node.js/PM2
git -C /var/www/projeto status  # Verificar alterações de código
gh run list                     # Pipelines do GitHub Actions
ngrok http 3000                 # Expor um serviço local
ss -ltn | grep LISTEN           # Ver portas abertas
nginx -T | grep server_name     # Ver domínios configurados
certbot certificates            # Monitorar validade do SSL
```

Cada ferramenta fala a língua dela. Como desenvolvedor, **você pensa em projetos**.

## 💡 A Solução

O **DevScope** nasce dessa dor: descobre seus projetos automaticamente e agrupa containers, Git, CI, tunnels, banco, API e saúde — tudo no contexto do projeto, direto no terminal:

```
┌───────────────────────────────────────────────────────────────────────┐
│ DevScope                 CPU 21%   RAM 54%   DISK 31%        14:32:01 │
├───────────────────────────────────────────────────────────────────────┤
│ SYSTEM OVERVIEW                                                       │
│ Uptime: 12d 4h  •  Load: 0.42  •  Docker: 8  •  RAM: 8192/16384 MB   │
├───────────────────────────────────────────────────────────────────────┤
│ PROJECTS (12)                                                         │
│   NAME              STATUS    BRANCH   CPU   RAM    PORTS             │
│ ● api               ● Run     main     12%   340M   :3000             │
│ ● frontend          ● Deg     develop   8%   128M   :5173             │
│ ○ worker            ● Stop    main      -     -     -                 │
├───────────────────────────────────────────────────────────────────────┤
│ Total: 12   Running: 8   Stopped: 3   Degraded: 1                     │
│ ↑↓ navigate  ENTER open  / filter  g git  c containers  ? help  q quit│
└───────────────────────────────────────────────────────────────────────┘
```

Dentro de cada projeto, os módulos ficam organizados por grupo:

| Grupo | Módulos |
|---|---|
| **WATCH** | Visão Geral · Metrics · Status |
| **SCOPE** | Git · Containers |
| **AUTOMATION** | GH Actions · Jenkins |
| **MANAGER** | Swarm · Kubernetes |
| **TUNNEL** | Ngrok · SSH Tunnel · CF Tunnel |
| **TOOLS** | Rotas · API · Database · WebSocket |

---

## 🎯 Por que DevScope?

* **Orientado a projeto** — não lista recursos soltos; tudo aparece no contexto do projeto certo.
* **TUI nativa** — binário único, leve, sem browser e sem daemon.
* **Operação no terminal** — logs, shell, compose, deploy, tunnels e SQL sem trocar de ferramenta.
* **Descoberta automática** — scan + detecção de stack; você abre e já vê o que está rodando.
* **Open source (MIT)** — feito para quem vive entre vários projetos ao mesmo tempo.

---

## ✨ Funcionalidades

### WATCH — Observabilidade

* 📂 **Descoberta Automática** — Varre `/var/www`, `/home`, `/opt` e caminhos configurados.
* 🏷️ **Detecção de Frameworks** — NestJS, Laravel, Django, Next.js, Vue, React, Nuxt, Go, Python, Rust, PHP, Java e outros.
* 📊 **Métricas do Sistema e do Projeto** — CPU, RAM, Disco, Swap, Load, Uptime e métricas agregadas por projeto.
* 🩺 **Status / Health Checks** — Probes HTTP/TCP, portas, domínios e SSL (Let's Encrypt) com status `Running`, `Degraded`, `Stopped`.
* 🔒 **Nginx & SSL** — Mapeamento de vhosts / proxy reverso e validade de certificados.

### SCOPE — Código e runtime

* 🐙 **Git nativo** — Branches, stage/unstage, commit, pull/push, merge, cherry-pick, diff colorido e abertura de PR no GitHub — tudo dentro do DevScope.
* 🐳 **Containers por projeto** — Correlação automática com Docker, stats, logs, env, config, shell (`exec`), start/stop/restart/pause/remove, portas e preview HTTP.
* 📦 **Docker Compose** — `up -d`, `down`, `restart` e criação de serviços no contexto do projeto.
* ⚙️ **PM2** — Workers Node vinculados a cada projeto.

### AUTOMATION — CI/CD

* ⚙️ **GitHub Actions** — Processes, runs, workflows, logs, YAML, trigger, re-run e login via `gh`.
* 🧱 **Jenkins** — Visão e operação de jobs no contexto do projeto.

### MANAGER — Orquestração

* 🐝 **Docker Swarm** — Services, nodes, tasks, stacks, networks, secrets, configs, scale, update, rollback e logs.
* ⎈ **Kubernetes** — Pods, deployments, services, manifests do projeto, apply/edit/delete, logs e scale via `kubectl`.

### TUNNEL — Exposição local

* 🌐 **Ngrok** — Tunnels, requests, histórico, domains e settings ligados ao projeto.
* 🔐 **SSH Tunnel** — Túneis SSH gerenciados no painel.
* ☁️ **Cloudflare Tunnel** — CF Tunnel no mesmo fluxo de operação.

### TOOLS — Dia a dia do desenvolvedor

* ⇄ **Rotas** — Detecta a stack, descobre endpoints (OpenAPI + parsers) e abre direto na aba API.
* 📡 **Cliente API** — HTTP no contexto do projeto: método, URL, headers, auth, body e response.
* 🗃️ **Database** — Lista tabelas e roda SQL em Postgres/MySQL detectados nos containers do projeto.
* ⚡ **WebSocket** — Connections, envio de frames, histórico, filtros e inspector.

### Operação geral

* ⚡ **Ações rápidas** — Terminal no projeto, OpenCode, deploy com confirmação, abrir URL no navegador.
* 🎨 **Temas** — `devscope`, dark, tokyo-night, catppuccin, rose-pine, solarized, dracula, nord, monokai, gruvbox, light, auto.
* 🔍 **Busca** — Filtro por nome (`/`) e fuzzy finder global (`Ctrl+P`).

### 🟡 Em Desenvolvimento

* [ ] Conexão e monitoramento multi-host via SSH
* [ ] Alertas e notificações via Webhooks (Slack, Discord)
* [ ] Demo em GIF na página principal

---

## ⌨️ Atalhos do Teclado

<details>
<summary>📂 <b>Dashboard Principal</b></summary>

| Tecla | Ação |
|---|---|
| `↑` / `↓` / `k` / `j` | Navegar entre projetos |
| `Enter` | Abrir detalhes do projeto selecionado |
| `/` | Filtrar projetos por nome |
| `Ctrl+P` | Busca global rápida (Fuzzy Finder) |
| `g` | Abrir direto na aba Git |
| `c` | Abrir direto na aba Containers |
| `Shift+E` | Abrir terminal no diretório do projeto |
| `Shift+O` | Abrir OpenCode no diretório do projeto |
| `T` | Escolher theme |
| `Ctrl+T` | Abrir a tela Relax (animações de terminal) |
| `r` | Atualizar dados do sistema |
| `?` | Abrir menu de ajuda |
| `q` | Sair do DevScope |

</details>

<details>
<summary>🧘 <b>Relax</b></summary>

Animações de terminal só para descansar a cabeça — nada roda nem é executado aqui.

| Tecla | Ação |
|---|---|
| `Ctrl+T` | Abrir / fechar a tela Relax (de qualquer lugar) |
| `↑` / `↓` / `Tab` | Trocar de game |
| `1` / `2` / `3` | Magic Cube / Asteroid / Cat |
| `Esc` | Voltar para a tela anterior |

</details>

<details>
<summary>📄 <b>Detalhes do Projeto</b></summary>

| Tecla | Ação |
|---|---|
| `Esc` | Voltar para o Dashboard |
| `Tab` / `Shift+Tab` | Próximo / módulo anterior (sidebar) |
| `h` | Ir para Status |
| `l` | Ir para Containers (logs no detalhe) |
| `D` | Executar script de Deploy (confirmação) |
| `Shift+U` / `Shift+D` / `R` | Compose up / down / restart |
| `o` | Abrir URL do projeto no navegador |

</details>

<details>
<summary>🐙 <b>Git</b></summary>

| Tecla | Ação |
|---|---|
| `←` / `→` / `h` / `l` | Alternar foco (Branches / Commits) |
| `a` / `A` | Stage arquivo / stage all |
| `c` | Novo commit |
| `n` / `d` / `R` | Criar / apagar / renomear branch |
| `Space` | Checkout de branch |
| `p` / `P` | Pull (`--ff-only`) / Push |
| `m` / `r` | Após pull divergente: merge (`--no-ff`) / rebase |
| `M` | Merge na branch atual |
| `enter` / `e` | Em conflito: abrir diff (o vs t) |
| `o` / `t` / `b` / `c` / `x` | Em conflito: ours / theirs / ambas / continue / abort |
| `Shift+C` / `Shift+V` | Copiar / colar commits (cherry-pick) |
| `o` | Abrir Pull Request no GitHub (fora de conflito) |
| `Enter` | Detalhe ou diff em tela cheia |
| `b` | Filtrar branches |
| `/` | Buscar no diff |

</details>

<details>
<summary>🐳 <b>Containers</b></summary>

| Tecla | Ação |
|---|---|
| `Enter` | Portas do container / preview HTTP |
| `m` / `l` | Detalhes (Logs, Stats, Env, Config…) |
| `Shift+E` | Shell (`exec`) no container |
| `s` / `r` / `p` | Stop / start·restart / pause·resume |
| `d` | Remover container (confirmação) |
| `n` | Novo serviço (Docker Hub ou YAML) |
| `A` | Todos os projetos + órfãos / só do projeto |
| `v` | Filtrar só containers Docker reais |
| `Shift+U` / `Shift+D` | Compose up / down |
| `f` | Seguir logs |
| `/` | Buscar nos logs |

</details>

<details>
<summary>⚙️ <b>GitHub Actions</b></summary>

| Tecla | Ação |
|---|---|
| `Enter` | Abrir Control Center / detalhe do processo |
| `[` / `]` ou `1`–`3` | Processes · Runs · Workflows |
| `c` / `d` / `t` | Criar / deletar / trigger |
| `R` | Re-run |
| `L` | Login `gh` |
| `o` | Abrir no GitHub |
| `r` | Refresh |

</details>

<details>
<summary>⎈ <b>Kubernetes</b></summary>

| Tecla | Ação |
|---|---|
| `Enter` | Abrir cliente / describe / YAML |
| `[` / `]` | Alternar kind: pods / deploy / svc / yaml |
| `n` / `N` | Namespace seguinte / anterior |
| `a` | Apply |
| `c` / `e` | Criar template / editar YAML |
| `d` | Delete (confirmação) |
| `l` | Logs do pod |
| `+` / `-` | Scale deployment |
| `r` | Refresh |

</details>

<details>
<summary>🐝 <b>Docker Swarm</b></summary>

| Tecla | Ação |
|---|---|
| `Enter` | Control Center / detalhes |
| `[` / `]` ou `1`–`8` | Services · Nodes · Tasks · Stacks · Networks · Secrets · Configs · Events |
| `s` / `u` / `c` | Scale / update / create service |
| `d` | Deploy stack do projeto |
| `l` | Logs do service |
| `R` / `b` | Force update / rollback |
| `i` | Swarm init |
| `t` / `T` | Join token worker / manager |

</details>

<details>
<summary>📡 <b>API</b></summary>

| Tecla | Ação |
|---|---|
| `Tab` | Ciclar Request → URL → Headers → Auth |
| `m` / `↑↓` | Alternar método HTTP |
| `e` | Editor do Body |
| `Enter` | Enviar request |
| `r` | Reenviar |
| `u` | Ciclar porta detectada do projeto |
| `a` | Auth (`none` / `bearer` / `basic`) |
| `[` / `]` | Alternar Body / Response |
| `/` | Buscar na Response |

</details>

<details>
<summary>🗃️ <b>Database</b></summary>

| Tecla | Ação |
|---|---|
| `Enter` | Abrir cliente / preview `SELECT * LIMIT 50` |
| `Tab` | Tables · SQL · Result |
| `e` | Editar SQL |
| `Ctrl+Enter` | Executar SQL |
| `[` / `]` | Trocar banco detectado |
| `r` | Recarregar tabelas |

</details>

<details>
<summary>⇄ <b>Rotas</b></summary>

| Tecla | Ação |
|---|---|
| `Enter` | Escanear rotas / abrir na aba API |
| `b` | Filtrar por path |
| `r` | Reescanear |

</details>

<details>
<summary>⚡ <b>WebSocket</b></summary>

| Tecla | Ação |
|---|---|
| `Enter` | Abrir Overview / conectar / enviar |
| `n` / `e` / `x` | Nova / editar / deletar connection |
| `c` / `d` | Conectar / desconectar |
| `0`–`3` | Overview · Messages · History · Settings |
| `f` | Filtrar frames |
| `/` | Buscar no payload |
| `Ctrl+L` | Limpar frames |

</details>

---

## 🛠️ Instalação Avançada

### Instalar uma versão específica

**Linux/macOS:**
```bash
DEVSCOPE_VERSION=0.1.0 curl -fsSL https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
$env:DEVSCOPE_VERSION="0.1.0"
irm https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.ps1 | iex
```

### Instalar em diretório personalizado

**Linux/macOS:**
```bash
DEVSCOPE_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
$env:DEVSCOPE_INSTALL_DIR="C:\Tools\devscope"
irm https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.ps1 | iex
```

### Download Direto (Manual)

Baixe o binário pré-compilado para a sua plataforma em [GitHub Releases](https://github.com/PirataZang/devscope/releases):

| Plataforma | Arquivo |
|---|---|
| 🐧 Linux x64 | `devscope_*_linux_amd64.tar.gz` |
| 🐧 Linux ARM64 | `devscope_*_linux_arm64.tar.gz` |
| 🍎 macOS x64 | `devscope_*_darwin_amd64.tar.gz` |
| 🍎 macOS Apple Silicon | `devscope_*_darwin_arm64.tar.gz` |
| 🪟 Windows x64 | `devscope_*_windows_amd64.zip` |
| 🪟 Windows ARM64 | `devscope_*_windows_arm64.zip` |

Cada release inclui um arquivo `checksums.txt` para verificar a integridade do download.

### Build from Source (requer Go 1.22+)

```bash
git clone https://github.com/PirataZang/devscope.git
cd devscope
make build        # Compila o binário em ./bin/devscope
make run          # Compila e executa localmente
make install-dev  # Compila e instala no PATH de desenvolvimento
```

Ou via `go install`:
```bash
go install github.com/devscope/devscope/cmd/devscope@latest
```
*Certifique-se de que `$GOPATH/bin` ou `$HOME/go/bin` esteja no seu `PATH`.*

---

## ⚙️ Configuração

O DevScope funciona sem configuração prévia. Para customizar, copie o arquivo de exemplo:

```bash
mkdir -p ~/.config/devscope
cp configs/devscope.example.yaml ~/.config/devscope/config.yaml
```

Parâmetros disponíveis ([configs/devscope.example.yaml](configs/devscope.example.yaml)):

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
  theme: devscope   # dark | tokyo-night | catppuccin | rose-pine | …
                    # solarized | dracula | nord | monokai | gruvbox | light | auto

health:
  timeout: 5s
  concurrent: 10

pinned:
  - /var/www/projeto
```

### Variáveis de Ambiente

Sobrescreva configurações com variáveis prefixadas com `DEVSCOPE_`:

```bash
DEVSCOPE_SCAN_PATHS=/var/www,/home/usuario/projetos devscope
```

---

## 🏗️ Arquitetura

```
Caminhos de Scan ──> Varredura do Disco ──> Detectores de Framework
                                                    │
                                                    ▼
Bubble Tea UI <── Snapshot Imutável <── Coletores (Docker, PM2, Git, Health, Nginx, …)
```

Para detalhes técnicos, consulte [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## 🤝 Contribuindo

Adoramos contribuições! Se você encontrou um bug ou quer propor melhorias:

* 🐛 Encontrou um bug? [Abra uma Issue de Bug](https://github.com/PirataZang/devscope/issues/new?template=bug_report.md)
* 💡 Tem uma ideia? [Solicite uma Feature](https://github.com/PirataZang/devscope/issues/new?template=feature_request.md)
* ⚙️ Novo framework? [Abra uma solicitação de detector](https://github.com/PirataZang/devscope/issues/new?template=detector_request.md)

---

## 📄 Licença

Este projeto está sob a licença [MIT](LICENSE).
