# DevScope

<p align="center">
  <strong>O "htop" dos seus projetos</strong> — visualize, monitore e opere todos os projetos e contêineres da sua VPS em uma única TUI (Interface de Terminal).
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

O DevScope faz um scan automático nos diretórios mais comuns (`/var/www`, `/home`, `/opt`) e exibe o painel interativo.

### 🔧 Outros Modos de Uso

```bash
devscope scan --json          # Snapshot completo do servidor em JSON (útil para automações)
devscope watch                # Painel com auto-refresh no terminal
devscope version              # Informações de versão e build
```

---

## 🔍 O Problema

Gerenciar uma VPS Linux típica exige monitorar dezenas de utilitários isolados:

```bash
docker ps -a                    # Ver contêineres rodando
docker stats --no-stream        # Monitorar uso de recursos
pm2 list                        # Processos Node.js/PM2
git -C /var/www/projeto status  # Verificar alterações de código
ss -ltn | grep LISTEN           # Ver portas abertas
nginx -T | grep server_name     # Ver domínios configurados
certbot certificates            # Monitorar validade do SSL
```

Cada ferramenta expõe uma unidade diferente. No entanto, como desenvolvedor, **você pensa em projetos**.

## 💡 A Solução

O **DevScope** resolve isso unificando o monitoramento. Ele descobre seus projetos automaticamente e agrupa tudo em uma única tela interativa:

```
┌───────────────────────────────────────────────────────────────────────┐
│ DevScope v0.1.0          CPU 21%   RAM 54%   DISK 31%        14:32:01 │
├───────────────────────────────────────────────────────────────────────┤
│ SYSTEM OVERVIEW                                                       │
│ Uptime: 12d 4h  •  Load: 0.42  •  Docker: 8  •  RAM: 8192/16384 MB   │
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

---

## ⚖️ Por que DevScope?

| Ferramenta | Unidade Principal | Como o DevScope Complementa/Ajuda |
|---|---|---|
| 🐳 [LazyDocker](https://github.com/jesseduffield/lazydocker) | Contêiner | Agrupa e exibe contêineres e recursos **por projeto** |
| 🐙 [LazyGit](https://github.com/jesseduffield/lazygit) | Repositório | Integração direta (abre o LazyGit com a tecla `L` no contexto do projeto) |
| 📊 `docker ps` / `htop` | Contêiner / Processo | Visão unificada orientada ao projeto (status, saúde, domínios, etc.) |
| 🌐 Portainer | Web UI | TUI nativa leve, binário único, roda sem necessidade de navegador |

---

## ✨ Funcionalidades

### 🟢 Disponível Agora
* 📂 **Descoberta Automática** — Varre os diretórios `/var/www`, `/home`, `/opt` e caminhos configurados.
* 🏷️ **Detecção Inteligente de Frameworks** — Suporte nativo para NestJS, Laravel, Django, Next.js, Vue, React, Nuxt, Go, Python, Rust, PHP, Java, etc.
* 📊 **Métricas do Sistema** — Monitoramento de CPU, RAM, Disco, Swap, Load e Uptime direto do host.
* 🐳 **Integração com Docker** — Status de contêineres, estatísticas em tempo real e correlação automática com seus projetos.
* 🐙 **Integração com Git** — Branch atual, status da working tree, histórico de commits e diff.
* ⚙️ **Suporte a PM2** — Identifica e lista workers vinculados a cada projeto.
* 🩺 **Health Checks** — Validação de endpoints via HTTP/TCP com status inteligente (`Running`, `Degraded`, `Stopped`).
* 🔒 **Nginx & SSL** — Mapeamento de domínios reversos e monitoramento de certificados Let's Encrypt.
* ⚡ **Ações Interativas** — Terminal no projeto, pausar/reiniciar/remover contêineres, logs em tempo real.

### 🟡 Em Desenvolvimento
* [ ] Conexão e monitoramento multi-host via SSH (v2.0)
* [ ] Alertas e notificações via Webhooks (Slack, Discord)
* [ ] Integração com CI/CD (GitHub Actions / GitLab CI)
* [ ] Demo em GIF na página principal

---

## ⌨️ Atalhos do Teclado

<details>
<summary>📂 <b>Dashboard Principal</b> (Clique para expandir)</summary>

| Tecla | Ação |
|---|---|
| `↑` / `↓` / `k` / `j` | Navegar entre projetos |
| `Enter` | Abrir detalhes do projeto selecionado |
| `/` | Filtrar projetos por nome |
| `Ctrl+P` | Busca global rápida (Fuzzy Finder) |
| `g` | Alternar diretamente para a aba Git |
| `c` | Alternar diretamente para a aba Containers |
| `Shift+E` | Abrir terminal no diretório do projeto |
| `r` | Atualizar dados do sistema manualmente |
| `?` | Abrir menu de ajuda |
| `q` | Sair do DevScope |

</details>

<details>
<summary>📄 <b>Detalhes do Projeto</b> (Clique para expandir)</summary>

| Tecla | Ação |
|---|---|
| `Esc` | Voltar para o Dashboard principal |
| `Tab` / `Shift+Tab` | Próxima / aba anterior |
| `1` – `6` | Atalhos para abas: Overview, Git, Containers, Health, Logs, Metrics |
| `L` | Abrir o **LazyGit** no contexto do projeto atual |
| `D` | Executar script de Deploy (com confirmação) |
| `o` | Abrir URL do projeto no navegador |
| `h` | Aba de monitoramento de Saúde (Health) |
| `l` | Aba de Logs |

</details>

<details>
<summary>🐙 <b>Controle Git</b> (Clique para expandir)</summary>

| Tecla | Ação |
|---|---|
| `←` / `→` / `h` / `l` | Alternar entre seções do Git |
| `b` | Filtrar branches |
| `Enter` | Ver detalhes de commits ou arquivos (somente leitura) |

</details>

<details>
<summary>🐳 <b>Controle de Contêineres</b> (Clique para expandir)</summary>

| Tecla | Ação |
|---|---|
| `Shift+E` | Entrar no Shell (`exec`) do contêiner |
| `p` | Pausar contêiner |
| `r` | Reiniciar contêiner |
| `d` | Remover contêiner (requer confirmação com `y`) |
| `m` | Ver logs estáticos |
| `f` | Seguir logs em tempo real (pausa com `p`) |

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
  theme: auto          # Opções: dark | light | auto

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
Caminhos de Scan (ScanPaths) ──> Varredura do Disco (Walk filesystem) ──> Detectores de Framework
                                                                               │
                                                                               ▼
Bubble Tea UI <── Snapshot Imutável <── Coletores (Docker, PM2, Git, Health, Nginx)
```

Para detalhes técnicos, consulte [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## 🤝 Contribuindo

Adoramos contribuições! Se você encontrou um bug ou quer propor melhorias, siga os passos em [CONTRIBUTING.md](CONTRIBUTING.md).

* 🐛 Encontrou um bug? [Abra uma Issue de Bug](https://github.com/PirataZang/devscope/issues/new?template=bug_report.md)
* 💡 Tem uma ideia? [Solicite uma Feature](https://github.com/PirataZang/devscope/issues/new?template=feature_request.md)
* ⚙️ Novo framework? [Abra uma solicitação de detector](https://github.com/PirataZang/devscope/issues/new?template=detector_request.md)

---

## 📄 Licença

Este projeto está sob a licença [MIT](LICENSE).
