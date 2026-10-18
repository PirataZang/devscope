<div align="center">

# ◆ DevScope

### O `htop` dos seus projetos

Containers, Git, CI, túneis, banco e saúde de **todos** os seus projetos —
numa tela só, dentro do terminal.

<p>
  <a href="https://github.com/PirataZang/devscope/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/PirataZang/devscope/ci.yml?branch=main&label=CI&style=for-the-badge&labelColor=0d1117" alt="CI"></a>
  <a href="https://github.com/PirataZang/devscope/releases"><img src="https://img.shields.io/github/v/release/PirataZang/devscope?label=release&style=for-the-badge&labelColor=0d1117&color=7c3aed" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=for-the-badge&labelColor=0d1117" alt="MIT"></a>
  <img src="https://img.shields.io/badge/go-1.24%2B-00ADD8?style=for-the-badge&labelColor=0d1117&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/linux%20%C2%B7%20macos%20%C2%B7%20windows-informational?style=for-the-badge&labelColor=0d1117&color=334155" alt="Plataformas">
</p>

</div>

## 🚀 Instalação

Um comando, um binário. Sem Node, sem Python, sem daemon.

<table>
<tr>
<th width="50%">🐧 Linux &nbsp;·&nbsp; 🍎 macOS</th>
<th width="50%">🪟 Windows (PowerShell)</th>
</tr>
<tr>
<td valign="top">

```bash
curl -fsSL https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.sh | bash
```

Detecta sistema e arquitetura (x64 / ARM64, Intel / Apple Silicon),
confere o checksum e instala em `/usr/local/bin` — ou em
`~/.local/bin`, já adicionado ao `PATH` do seu `.bashrc` / `.zshrc`.

</td>
<td valign="top">

```powershell
irm https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.ps1 | iex
```

Instala em `%USERPROFILE%\.devscope\bin` e coloca a pasta no `PATH`
do usuário. Abra um terminal novo e pronto.

</td>
</tr>
</table>

Depois é só rodar:

```bash
devscope               # abre a TUI
devscope scan --json   # snapshot da máquina em JSON, para automação
devscope watch         # painel com auto-refresh, sem TUI interativa
devscope version       # versão, commit e data de build
```

<sub>Versão específica, outra pasta, download manual ou build from source? → [Instalação avançada](#-instalação-avançada)</sub>

---

<p align="center">
  <img src="docs/media/overview.gif" alt="DevScope: lista de projetos, filtro, busca fuzzy, módulos do projeto, ajuda e temas" width="100%">
</p>

<div align="center">

**[Git](#-git-sem-sair-do-terminal)** ·
**[Docker](#-docker-por-projeto)** ·
**[Por que existe](#-por-que-existe)** ·
**[Módulos](#-15-módulos-um-contexto)** ·
**[Atalhos](#-atalhos-essenciais)** ·
**[Preferências](#-preferências)** ·
**[Arquitetura](#-como-funciona)**

<sub>Todas as gravações são do DevScope de verdade, rodando contra projetos de demonstração.</sub>

</div>

---

## ⚡ Em 30 segundos

<div align="center">
<table>
<tr>
<td width="25%" align="center">

### 📦
**Bin