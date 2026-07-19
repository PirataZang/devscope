# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/),
e este projeto segue o [Versionamento Semântico](https://semver.org/).

## [Unreleased]

### Added
- **Aba API (cliente HTTP embutido)**
  - Cliente fullscreen no estilo Postman/LazyDocker (aba `7`)
  - Métodos coloridos: GET, POST, PUT, PATCH, DELETE
  - Painéis de Request, URL, Headers e Auth (Bearer / Basic / none)
  - Editor de Body com indentação, navegação por linhas e expansão de `{|}` / `[|]`
  - Response viewer com status, tempo, headers, JSON formatado, scroll e busca (`/`)
  - Sugestão automática de `http://localhost:<porta>` a partir das portas do projeto
  - Histórico leve em memória dos últimos requests da sessão
  - Highlight de JSON no body e na response
- **Sidebar do projeto**
  - Rail lateral com brand, health e branch
  - Navegação agrupada (SCOPE / WATCH / TOOLS)
  - Badges e medidores no footer (CPU/RAM)
- **Carregamento assíncrono do projeto**
  - Enrichment de Git e Docker em background ao abrir o projeto
- **Melhorias na aba Git**
  - Diff colorido (adições/remoções, estilo LazyGit)
  - Navegação entre arquivos do commit (`n` / `p`)
  - Expandir mensagem do commit (`m`)
  - Busca no diff (`/`) e scroll horizontal (`←` / `→`)
  - Testes de prompt, histórico de branches e navegação
- **Melhorias na aba Containers**
  - Detalhe do container expandido (logs, stats, env, config)
  - Busca nos logs (`/`) e scroll horizontal (`,` / `.`)
  - Atalhos refinados de lifecycle (start/restart, pause/resume)

### Changed
- Atalhos de abas do projeto: `1`–`7` (Overview, Git, Containers, Health, Logs, Metrics, **API**)
- Layout do projeto passa a usar sidebar + painel de conteúdo
- README atualizado com atalhos da aba API, Git e Containers
- Collectors Docker/Git e manager refinados para suporte ao enrichment e UI

### Fixed
- Ajustes de renderização de texto largo com helper `sliceColumns`
- Estilos de diff Git e tema para melhor contraste de adições/remoções

---

## [0.1.2] - 2026-07-15

### Added
- **Aba Git completa**
  - Gerenciamento de branches (checkout, criar, renomear, apagar, marcar origem)
  - Histórico de commits com visualização de detalhes (mensagem e arquivos alterados)
  - Cherry-pick: seleção individual/range de commits, copiar e colar entre branches
  - Pull/Push, merge de branch, abrir Pull Request no GitHub
  - Filtro de branches e working tree
- **Aba de Containers**
  - Listagem e monitoramento de containers Docker
  - Detalhes: logs (com follow), stats, env, config
  - Ciclo de vida: start/restart, stop, pause/resume, remover
  - Shell interativo dentro do container
  - Docker compose up/down/restart

### Fixed
- **Tela de ajuda/atalhos não abria dentro de um projeto**
  - O `?` só funcionava no dashboard; dentro de um projeto a telinha de
    comandos não aparecia. Agora o `?` abre a ajuda com todos os atalhos
    (incluindo os da aba Git) em qualquer view de projeto. Feche com `esc` ou `?`.

## [0.1.1] - 2026-07-15

### Added
- Integração Git inicial: navegação de branches, histórico de commits e views de detalhe

## [0.1.0] - 2026-07-15

### Added
- Primeira versão tagueada
- Scripts de instalação cross-platform (bash / PowerShell)
- Validação SHA256 de releases
- Descoberta de projetos, métricas do host, Docker, health checks e TUI base
