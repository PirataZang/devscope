# DevScope

<p align="center">
  <strong>O "htop" dos seus projetos</strong> — visualize, monitore e opere todos os projetos e contêineres da sua VPS em uma única TUI (Interface de Terminal).
</p>

<p align="center">
  <a href="https://github.com/PirataZang/devscope/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/PirataZang/devscope/ci.yml?branch=main&label=CI&style=flat-square" alt="CI"></a>
  <a href="https://github.com/PirataZang/devscope/releases"><img src="https://img.shields.io/github/v/release/PirataZang/devscope?label=release&style=flat-square" alt="Release"></a>
  <a href="https://github.com/PirataZang/devscope/blob/main/LICENSE"><img src="img.shields.io/badge/license-MIT-blue.svg?style=flat-square" alt="License MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.22+-00ADD8?logo=go&logoColor=white&style=flat-square" alt="Go 1.22+"></a>
</p>

---

## ⚡ Instalação Rápida

Escolha o comando correspondente ao seu sistema operacional para instalar o DevScope instantaneamente:

### 🐧 Linux & 🍎 macOS
```bash
curl -fsSL https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.sh | bash
```

### 🪟 Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.ps1 | iex
```

> 💡 *Deseja customizar a instalação (mudar diretório, versão ou instalar via Go)? Veja a seção [Instalação Avançada](#-instalação-avançada).*

---

## 🚀 Como Usar (Quick Start)

Após instalar, basta rodar o comando abaixo no seu terminal. O DevScope fará um scan automático nos diretórios mais comuns (`/var/www`, `/home`, `/opt`):

```bash
devscope
```

### 🔧 Outros Modos de Uso (Sem TUI)
Se você precisar extrair dados para automações ou monitoramento rápido:
```bash
devscope scan --json          # Retorna um snapshot completo do servidor em formato JSON
devscope watch                # Executa o painel com auto-refresh direto no terminal
devscope version              # Exibe informações detalhadas de versão e build
```

---

## 🔍 O Problema

Gerenciar uma VPS Linux típica exige monitorar dezenas de utilitários isolados:
```bash
docker ps -a                 # Para ver os contêineres rodando
docker stats --no-stream     # Para monitorar uso de recursos de Docker
pm2 list                     # Para processos Node.js/PM2
git -C /var/www/projeto status # Para verificar alterações de código
ss -ltn | grep LISTEN        # Para ver quais portas estão abertas
nginx -T | grep server_name  # Para ver domínios configurados
certbot certificates         # Para monitorar validade do SSL
```
Cada ferramenta expõe uma unidade diferente (contêiner, processo, porta). No entanto, como desenvolvedor, **você pensa em projetos**.

## 💡 A Solução

O **DevScope** resolve isso unificando o monitoramento. Ele descobre seus projetos automaticamente e agrupa todas as informações em uma única tela interativa de terminal (TUI).

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
* 📊 **Métricas do Sistema** — Monitoramento de uso de CPU, RAM, Disco, Swap, Load e Uptime direto do host.
* 🐳 **Integração com Docker** — Status de contêineres, estatísticas de uso em tempo real e correlação automática com seus projetos.
* 🐙 **Integração com Git** — Detecção de branch atual, status da working tree (arquivos modificados), histórico de commits e visualização de diff.
* ⚙️ **Suporte a PM2** — Identifica e lista workers vinculados a cada projeto.
* 🩺 **Health Checks** — Validação ativa de endpoints via HTTP/TCP com status inteligente (`Running`, `Degraded`, `Stopped`).
* 🔒 **Nginx & SSL** — Mapeamento de domínios reversos e monitoramento de expiração de certificados Let's Encrypt.
* ⚡ **Ações Interativas** — Abrir terminal no projeto ou dentro de contêineres, pausar, reiniciar, remover recursos e monitorar logs em tempo real (*follow*).

### 🟡 Em Desenvolvimento
* [ ] Conexão e monitoramento multi-host via SSH (v2.0)
* [ ] Alertas e notificações via Webhooks (Slack, Discord)
* [ ] Integração com esteiras de CI/CD (GitHub Actions / GitLab CI)
* [ ] Gravação demonstrativa em GIF na página principal

---

## ⌨️ Atalhos do Teclado

Para facilitar a navegação rápida, o DevScope é totalmente operável pelo teclado:

<details>
<summary>📂 <b>Atalhos do Dashboard Principal</b> (Clique para expandir)</summary>

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
<summary>📄 <b>Atalhos de Detalhes do Projeto</b> (Clique para expandir)</summary>

| Tecla | Ação |
|---|---|
| `Esc` | Voltar para o Dashboard principal |
| `Tab` / `Shift+Tab` | Ir para a próxima / aba anterior |
| `1` – `6` | Atalhos para abas: Overview, Git, Containers, Health, Logs, Metrics |
| `L` | Abrir o **LazyGit** no contexto do projeto atual |
| `D` | Executar script de Deploy (com confirmação) |
| `o` | Abrir URL do projeto no navegador |
| `h` | Acessar aba de monitoramento de Saúde (Health) |
| `l` | Acessar aba de Logs |

</details>

<details>
<summary>🐙 <b>Atalhos de Controle Git</b> (Clique para expandir)</summary>

| Tecla | Ação |
|---|---|
| `←` / `→` / `h` / `l` | Alternar entre seções do Git |
| `b` | Filtrar branches |
| `Enter` | Ver detalhes de commits ou arquivos (somente leitura) |

</details>

<details>
<summary>🐳 <b>Atalhos de Controle de Contêineres</b> (Clique para expandir)</summary>

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

### Instalação via Go
Se você tem o Go instalado no seu ambiente (versão 1.22+):
```bash
go install github.com/devscope/devscope/cmd/devscope@latest
```
*Certifique-se de que o diretório `$GOPATH/bin` ou `$HOME/go/bin` esteja configurado na sua variável de ambiente `PATH`.*

### Compilação Manual (Build from Source)
```bash
git clone https://github.com/PirataZang/devscope.git
cd devscope
make run          # Compila e executa localmente (ambiente de desenvolvimento)
make install-dev  # Compila e move o binário para o seu PATH de desenvolvimento
make build        # Apenas compila o binário em ./bin/devscope
```

### Download Direto
Você também pode baixar os binários compilados manualmente acessando a página de [GitHub Releases](https://github.com/PirataZang/devscope/releases). Os arquivos acompanham um arquivo `checksums.txt` para você validar a integridade do download.

---

## ⚙️ Configuração

O DevScope roda totalmente sem configuração prévia, mas você pode customizar seu comportamento criando um arquivo de configuração.

Para começar, copie o modelo padrão:
```bash
mkdir -p ~/.config/devscope
cp configs/devscope.example.yaml ~/.config/devscope/config.yaml
```

O arquivo de configuração suporta os seguintes parâmetros ([configs/devscope.example.yaml](configs/devscope.example.yaml)):

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
Você também pode sobrescrever as configurações usando variáveis com o prefixo `DEVSCOPE_`. 
*Exemplo:* `DEVSCOPE_SCAN_PATHS=/var/www,/home/usuario/projetos`

---

## 🏗️ Arquitetura

O funcionamento interno segue o seguinte fluxo de dados unidirecional:
```
Caminhos de Scan (ScanPaths) ──> Varredura do Disco (Walk filesystem) ──> Detectores de Framework
                                                                               │
                                                                               ▼
Bubble Tea UI <── Snapshot Imutável <── Coletores (Docker, PM2, Git, Health, Nginx)
```
Para saber mais detalhes técnicos, consulte a nossa documentação em [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## 🤝 Contribuindo

Adoramos contribuições! Se você encontrou um bug ou quer propor melhorias, siga os passos descritos em [CONTRIBUTING.md](CONTRIBUTING.md).

* 🐛 Encontrou um bug? [Abra uma Issue de Bug](https://github.com/PirataZang/devscope/issues/new?template=bug_report.md)
* 💡 Tem uma ideia de funcionalidade? [Solicite uma Feature](https://github.com/PirataZang/devscope/issues/new?template=feature_request.md)
* ⚙️ Quer adicionar suporte a um novo framework? [Abra uma solicitação de detector](https://github.com/PirataZang/devscope/issues/new?template=detector_request.md)

---

## 📄 Licença

Este projeto está sob a licença [MIT](LICENSE).
