# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/),
e este projeto segue o [Versionamento Semântico](https://semver.org/).

## [Unreleased]

### Added
- **Módulo Cloudflare (grupo REDE)** — os domínios da sua conta vistos de dentro do DevScope, só leitura
  - Abre na lista de domínios, como no painel da Cloudflare: estado, plano, a semana de visitantes num minigráfico e a tendência, com os deste projeto no topo; `enter` abre o domínio e `esc` volta para a lista
  - **1 Tráfego**: visitantes e páginas comparados com o período anterior (7 ou 30 dias, `←→`), cache, respostas 2xx–5xx numa barra, ameaças, países, gráfico das últimas 24 horas hora a hora, uma barra por dia ("hoje", "ontem", pico e erros 5xx do dia), páginas mais acessadas e caminhos com erro
  - **2 DNS**: os registros, para onde apontam e quais passam pelo proxy
  - **3 Segurança**: modo SSL explicado em palavras, HTTPS forçado, TLS mínimo, validade do certificado, avisos do Security Center e vencimento do registro do domínio (RDAP, inclusive registro.br)
  - `w` abre o site, `o` abre a mesma tela no painel da Cloudflare
  - Só aparece no projeto que tem um domínio da conta: site do Cloudflare Pages ligado ao mesmo repositório do GitHub, endereço do domínio no `.env`, `CNAME`, `package.json`, `astro.config`, `nginx.conf` ou `wrangler.toml`, ou vínculo feito à mão com `v` (grava `.devscope/cloudflare.txt`)
  - `devscope cloudflare login|status|logout` guarda e confere o API Token (ou `CLOUDFLARE_API_TOKEN`)

- **Cloudflare no bot do Telegram** — a máquina conectada (`devscope bot`) vigia os domínios dos projetos a cada 5 minutos e avisa
  - 📈 pico de visitas: a última hora com 3× o normal (e pelo menos 50 visitantes)
  - 🔥 pico de erros 5xx: 5% das requisições ou mais, com aviso de volta ao normal
  - certificado a menos de 14 dias e registro do domínio a menos de 30 dias de vencer
  - `/cloudflare` e o botão ☁️ Cloudflare no painel do projeto mostram no Telegram as visitas da semana contra a anterior, o desenho dos dias e das 24 horas com o pico, países, páginas mais acessadas, caminhos com erro e o SSL; os avisos da Cloudflare podem ser desligados em /avisos (precisa do hub atualizado)

- **Releases prepara o repositório** — ao pedir o release sem workflow de tag ou sem CHANGELOG, o DevScope oferece criar o que faltar antes (também com `w`)
  - `.github/workflows/release.yml` pelo projeto: com `.goreleaser.yaml`, goreleaser (binários por sistema); sem, GitHub Release com a mensagem da tag — a seção do CHANGELOG — como notas
  - `CHANGELOG.md` no formato Keep a Changelog com `## [Unreleased]` e **instruções para IA** num comentário invisível no GitHub: como levantar as mudanças (`git log` desde a última tag), agrupar, escrever pelo efeito para o usuário, escolher a versão e renomear a seção na hora de lançar — no formato que o Releases lê
  - Commita só esses arquivos e envia a branch, porque o GitHub roda o workflow que está no commit da tag; o resto das suas alterações fica como estava
  - Recusou: o próximo `enter` lança sem eles; arquivo existente nunca é sobrescrito

- **Publicar projeto no nginx (aba Nginx, `p` → `enter`)** — um projeto de `~/projetos` vira `sites/<projeto>.inc` no nginx-main sem digitar caminho nem porta
  - Detecta porta do back-end (`BACKEND_PORT` no `.env` → compose → container no ar), build do front e perfil (`saas`, `front`, `api`), mostrando de onde veio cada valor; com duas candidatas pede para escolher em vez de chutar
  - Checagens antes de publicar: conflito com outra `.inc` do hub (bloqueia, `o` abre a existente), `base` do Vite, `VITE_API_URL`, build com `/assets/` na raiz, `APP_URL` e `TrustProxies` do Laravel
  - Uma tela de revisão com a `.inc` pronta (e o diff ao republicar); `e` edita slug, porta, dist, perfil e limites, `+` adiciona rota extra como o `/portal/`
  - Publica com escrita atômica e `.inc.bak`, `nginx -t` (recusou: desfaz e não recarrega, com a linha destacada), reload e smoke test dos três caminhos; `c` copia a URL (OSC52 via SSH)
  - No próprio nginx-main, `p` pergunta qual projeto publicar e mostra quais já estão no ar
  - `devscope nginx publish|unpublish <projeto>` faz o mesmo sem a TUI, com `--dry-run`; sai com erro se bloquear, se o `nginx -t` recusar ou se o smoke test falhar
  - Ambiente lido do compose e do `conf.d/` do nginx-main; o que precisar fixar vai na seção `Nginx:` do `user_config.txt`

### Changed
- **O projeto aberto em destaque na barra lateral** — o nome vira uma faixa cheia na cor do módulo, com a pasta embaixo; a marca do DevScope desce para uma linha discreta. Em terminal baixo, a lista de módulos rola e o projeto fica fixo no topo (antes ele rolava para fora da caixa)
- **Releases documentado na própria tela** — nova seção COMO FUNCIONA com cada peça que o módulo usa (tags `vX.Y.Z`, `CHANGELOG.md`, workflow de tag, `gh`, remoto `origin`), se o repositório aberto tem e para que serve; o que falta vem com o formato ou o comando para configurar, marcado como opcional ou obrigatório
- Repositório sem remoto `origin` agora bloqueia o lançamento com o motivo, em vez de falhar no push
- README ganhou a seção "Releases" com os requisitos e o que bloqueia ou só avisa

### Fixed
- A versão aparecia como `vv1.7.4` na barra lateral e na tela inicial
- **A lista de projetos acompanha o disco sem reabrir o DevScope** — antes o disco só era varrido na abertura
  - Pasta apagada some da tela inicial em até 2 segundos
  - Projeto novo aparece em até 10 segundos (`refresh.scan_interval` passou de 60s para 10s)
  - Troca de branch feita no terminal ou no editor aparece na coluna BRANCH
- Digitar `q` no formulário de nova rota do Nginx fechava o DevScope

## [1.7.0] - 2026-10-18

### Added
- **Módulo Worktrees (CÓDIGO)** — `git worktree` sem sair do DevScope, simplificado para o que se usa todo dia
  - Lista com branch, pasta, commit e estado (`limpa`, `N alterações`, `travada`, `pasta sumiu`); `◆` marca a worktree aberta, e o detalhe abaixo mostra o caminho inteiro, o assunto do commit e o motivo da trava ou da poda
  - `n` cria pedindo só o nome da branch: a pasta é sempre irmã da principal (`<repo>-<branch>`), branch existente (local ou remota) faz checkout e nome novo cria a branch a partir da atual
  - `d` remove com uma confirmação só — com alteração pendente ela diz quantas se perdem e já usa `--force`; a principal e as travadas não se removem daqui
  - `enter` abre a worktree como projeto (Git, Containers e o resto passam a olhar para a pasta dela); `shift+E` e `ctrl+O` abrem terminal e agente de IA nela; `p` poda registros de pastas apagadas à mão
  - Aparece em todo repositório git; lock/unlock, move e repair ficam no terminal
- **Módulo Tarefas (EXECUÇÃO)** — os comandos que o projeto já declara, numa lista só: alvos do `Makefile`, receitas do `justfile`, tarefas do `Taskfile.yml` e scripts do `package.json` e do `composer.json`
  - `enter` roda no terminal de verdade — servidor de dev, cor e pergunta funcionam; `ctrl+c` para só a tarefa, e a saída fica na tela até o `enter` que volta ao DevScope
  - A lista guarda o resultado da última execução (`ok · 3s`, `saiu 2 · 14s`, `parada`), com glifos distintos para ler sem cor
  - Descrição tirada do comentário do autor (`## desc` na linha do alvo ou `#` logo acima, `desc:` do Taskfile) ou do próprio comando do script
  - Scripts de Node rodam com o gerenciador do lockfile (pnpm, yarn, bun ou npm); alvos internos ficam de fora (`.PHONY`, regras de padrão, receitas `_privadas`, tarefas `internal`)
  - Aparece quando o projeto tem algum desses arquivos
- **Conflito de portas entre projetos** — porta do host reivindicada por mais de um projeto aparece com aviso no resumo do dashboard e na lista de portas da abertura dos módulos (só um processo consegue escutar cada porta)
- **Módulo Releases (CÓDIGO)** — lançar uma versão sem esquecer passo
  - Mostra a última tag, quantos commits entram e as três próximas versões (patch · minor · major); a sugerida é a que já tem seção no CHANGELOG
  - VERIFICAÇÕES antes de lançar: alteração sem commit e commit não enviado bloqueiam; branch fora da padrão, remoto à frente e CHANGELOG sem a seção só avisam
  - `enter` cria a tag anotada no HEAD com a seção do CHANGELOG como mensagem (sem seção, a lista de commits) e envia só a tag; se o push falha, a tag local é desfeita
  - Depois do push acompanha ao vivo o workflow que a tag dispara (via `gh`) até publicar ou falhar; a lista de releases anteriores mostra o resultado de cada uma
- **Pendências de git em todos os projetos** — o dashboard responde "onde eu deixei coisa pela metade?" sem abrir um por um
  - Na coluna BRANCH: `±3` alterações locais, `↑2` commits a enviar, `↓1` a receber; na barra, quantos projetos têm pendência
  - `p` mostra só os projetos com pendência; `esc` volta para todos
  - Medido a cada `git_interval` (30s) com um `git status` por projeto, sem travar o index do git que você estiver usando
- **Portas (`shift+P` na tela inicial)** — quem está usando cada porta da máquina
  - Agrupada por dono: um cabeçalho por projeto (nome, quantas portas, pasta), depois composes que não estão na lista de projetos, programas e portas do sistema
  - Porta → processo → projeto: pelo container que publica a porta (inclusive de compose fora da lista, pela pasta dele), pela pasta onde o processo roda ou pela porta declarada no compose
  - Cores para o que importa: `rede` em amarelo (acessível por outras máquinas) e `local` em verde, container em azul, e porta disputada entre projetos em vermelho com `⚠ também em <projeto>`
  - `x` encerra com a ação certa para o dono (`docker stop` no container, SIGTERM no processo), com confirmação; `enter` abre o projeto, `o` abre no navegador
  - Mostra se a porta está aberta para a rede ou só local; no Linux lê `/proc` direto, no macOS usa o `lsof`
- **Disco na Visão Geral** — caixa DISCO com o tamanho das pastas que o projeto gera (`node_modules`, `vendor`, `target`, `.next`, `.venv`…)
  - `x` apaga todas com uma confirmação que diz o que sai e o comando que traz de volta
  - Num repositório git só entra pasta que o git ignora: `vendor/` commitado ou `build/` versionado nunca aparecem
  - `r` na Visão Geral agora atualiza de verdade (antes a caixa AÇÕES prometia e não fazia nada)

### Changed
- README refeito: instalação primeiro, tour em GIF da visão geral, do Git e do Docker, tabelas de atalhos e instalação avançada no fim
- Gravação das demos reproduzível (fitas VHS, `record.sh`, `zoom.py`) com projetos de demonstração descartáveis, no lugar do antigo `docs/demo.tape`
- O pacote de release leva só `docs/*.md` — GIFs e vídeos ficam fora do download

### Fixed
- Leitura do compose só considera porta solta dentro de `ports:` — `retries: 5` e `interval: 5s` não aparecem mais como porta 5; portas acima de 65535 são descartadas

### Removed
- **Aba Rotas** e o pacote `routeutil` — não fazia mais sentido no DevScope
- **Aba SSH Tunnel** e o pacote `sshutil` — não funcionava de forma confiável
- **Aba WebSocket** e o pacote `wsutil` — sem finalidade no DevScope; a dependência `gorilla/websocket` saiu junto

## [1.6.9] - 2026-09-25

> Acumula o que saiu entre a 1.6.4 e a 1.6.9.

### Added
- **Aba API repaginada no padrão Postman/Thunder Client** — a tela foi refeita inteira, no padrão de telas do projeto (cabeçalho, régua de abas numerada, painéis e barra de comandos larga)
  - **Linha de requisição** no topo, como no Postman: método, URL inteira e o botão de enviar numa linha só. Antes a URL aparecia duas vezes (cabeçalho + caixa `[2]-URL`) e em nenhuma das duas dava pra ver a query
  - **Requisição e resposta visíveis juntas.** Antes `Body` e `Response` disputavam o mesmo painel por `[` e `]` — ver o corpo enviado e a resposta ao mesmo tempo era impossível
  - **Abas numeradas** `1 PARAMS · 2 HEADERS · 3 AUTH · 4 BODY · 5 RESPOSTA`, com contador no rótulo (params ativos, headers ativos, código da resposta) pra saber o que está ligado sem abrir a aba; a aba 5 abre a resposta em tela cheia
  - **Tabela de params** nova, ligada à URL nos dois sentidos: colar um link vira linhas na hora, editar a linha reescreve a query, e `space` desliga o parâmetro **sem apagá-lo** (era preciso editar a query à mão dentro da URL)
  - **Headers também em tabela**, com `n` nova linha, `d` apagar e `space` ligar/desligar; linha desligada é serializada como comentário, que o cliente HTTP já ignorava
  - **Auth** em controle segmentado (nenhuma · bearer · basic) em vez de caixa empilhada, com o segredo mascarado
  - Resposta com **tamanho** (`1.2 kB`) além de status e tempo, numeração de linha, realce de JSON, busca e `h` alternando corpo/cabeçalhos
  - **`y` copia o comando `curl` equivalente** (método, URL, headers ativos, auth e corpo)
  - Foco em três paradas só (`tab`: URL → aba → resposta), no lugar de quatro caixas à esquerda mais um painel à direita onde `tab` fazia coisas diferentes conforme o lugar
  - A coluna vertical `AÇÕES` (17 colunas roubadas do conteúdo) deu lugar à barra de comandos larga do padrão, com os atalhos mudando conforme o foco
  - Edição agora é explícita (`u` na URL, `e` no campo, `enter` na célula da tabela): digitar em cima de um campo para começar a editar conflitava com as teclas de comando, que é a raiz da bagunça anterior
- **Relax: cena `Smoke`** — um gerador solta fumaça colorida de duas cores, elas se enrolam uma na outra, a coluna sobe, abre e dissipa; aí sai outra, com duas cores novas
  - Ciclo de 15s: 5s de jato e o resto até não sobrar nada (um desvanecimento de ciclo garante o palco limpo pro lançamento seguinte)
  - Duas cores sorteadas por lançamento, a 60°–180° uma da outra, e a fumaça é a rampa entre elas pelo caminho curto do círculo de matiz; a válvula alterna em fita, e cada baforada caminha pro meio da rampa conforme envelhece
  - Turbulência por **curl de ruído** (rotacional de um campo de ruído, livre de divergência): a fumaça enrola em vez de inflar como spray. O eixo vertical entra bem podado — solto, ele vira corrente descendente parada em cima da coluna e a fumaça não sobe
  - Densidade virando opacidade por absorção (Beer-Lambert) e opacidade virando densidade de pontos por meio-tom: o miolo fecha a célula, o meio fica em meio-tom e a borda esgarça em pontinhos, com um cálculo só
  - Uma em três baforadas é farrapo (menor, mais leve, mais turbulenta), e cada uma tem empuxo próprio — é o que estica a coluna na vertical em vez de fazê-la subir em bloco
- **Relax: cena `Dragon`** — um dragão de perfil batendo asa contra a lua, a 30fps
  - Anatomia procedural (tronco, pescoço, cabeça, cauda, pernas e duas asas com braço, antebraço, quatro dedos e o pano entre eles), não sprite pronto
  - Golpe assimétrico (38% do ciclo é a descida), asa que dobra no meio da subida, pano que atrasa em relação ao osso e corpo que sobe depois do golpe
  - Rugido de vez em quando: a boca abre antes do fogo, o jato sai em rampa de densidade ASCII (`. : - = + * # % @`), as brasas sobem como `@ * + . '` e o rugido sai escrito letra por letra
  - Voo batido e planado se alternando; serra, nuvens, lua e estrelas ao fundo
  - Movimento contínuo (asa, balanço, cauda, chama, trajetória de brasa) é função do tempo de render, não do passo de simulação — cada um dos ~30 frames por segundo desenha movimento novo
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

### Changed
- `.devscope/` agora é criado por um helper compartilhado (`devscopeutil.EnsureDir`), reaproveitado por `cfutil`, `jenkinsutil`, `ngrokutil`, `sshutil`, `wsutil` e o novo `dbutil`
- `.gitignore` passa a ignorar `.devscope/` (evita versionar config e credenciais locais do projeto)

### Fixed
- Detecção de processos `cloudflared` não reconhecia binário de release baixado sem renomear (`cloudflared-linux-amd64`)

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

[Unreleased]: https://github.com/PirataZang/devscope/compare/v1.7.0...HEAD
[1.7.0]: https://github.com/PirataZang/devscope/compare/v1.6.9...v1.7.0
[1.6.9]: https://github.com/PirataZang/devscope/compare/v1.6.3...v1.6.9
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
