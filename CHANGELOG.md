# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/),
e este projeto segue o [Versionamento Semântico](https://semver.org/).

## [Unreleased]

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
  - O `?` só funcionava no dashboard; dentro de um projeto (aba Git) a telinha de
    comandos não aparecia. Agora o `?` abre a ajuda com todos os atalhos
    (incluindo os da aba Git) em qualquer view de projeto. Feche com `esc` ou `?`.

## [0.1.1] - 2026-07-15

Versão base anterior.

## [0.1.0] - 2026-07-15

Primeira versão tagueada.
