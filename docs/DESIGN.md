# DevScope — Padrão de Telas (TUI)

> Como uma tela do DevScope deve ser desenhada. Vale para **todo módulo novo** e
> para toda tela reformulada. O que está aqui foi extraído das telas já
> convertidas — Início, Git, Docker, detalhe de container, GH Actions, ngrok,
> Cloudflared, SSH, Swarm, Kubernetes e Nginx.

O DevScope é uma TUI em Go com Bubble Tea. Especificação que chega em termos de
web (card, modal, hover, breakpoint) precisa ser traduzida antes de virar
código: aqui a unidade é a **célula do terminal**, o refresh é um **tick de
10 fps** e não existe rolagem infinita nem hover.

---

## 1. Princípios

| Princípio | O que significa na prática |
|-----------|----------------------------|
| **Uma informação, um lugar** | Nome, status e caminho do projeto aparecem **uma vez** por tela. Repetir na barra lateral, no cabeçalho e num box "DETALHES" é o erro mais comum. |
| **A tela inteira é o painel** | Nada de moldura arredondada externa: ela custa 2 colunas e 2 linhas e não separa nada quando o conteúdo ocupa tudo. |
| **Conteúdo ganha da decoração** | Se um elemento não responde a uma pergunta que o usuário faria, ele sai. Card com um número só é decoração. |
| **A estrutura aparece sozinha** | Alinhamento e realce por tipo de conteúdo, não parágrafos de legenda. |
| **Nada de placeholder** | "notas locais em breve", "MÓDULOS ATIVOS (vazio)" não entram no binário. Se não há dado, o estado vazio diz o que fazer. |
| **O glifo diz, a cor reforça** | Screenshot parado, terminal sem cor e daltonismo continuam legíveis: dois estados nunca podem ter o mesmo glifo no mesmo quadro. |

---

## 1.1 A fundação (`internal/ui/foundation.go`)

Toda tela é desenhada com as mesmas peças. Elas moram num arquivo só — antes o
painel estava em `api_tab.go`, o corte com consciência de ANSI em `git_tab.go` e
o espaçador em `dashboard.go`, e cada tela nova copiava a de onde tinha olhado
por último.

**Escolha na ordem, da mais leve para a mais pesada. Só desça um degrau quando o
de cima não separar:**

| # | Peça | Custo | Quando |
|---|------|-------|--------|
| 1 | `padRightVisible` · `joinWithSpacer` | 0 linhas | alinhar, empurrar contexto para a direita |
| 2 | `StyleSection` · `StyleMuted` · `keyHint` | 0 linhas | cor e peso já separam |
| 3 | `sectionHeader(título, largura, cor)` | 1 linha | agrupar sem caixa — **tente sempre isto antes da caixa** |
| 4 | `rule(largura)` · `ruleColored(largura, cor)` | 1 linha | separar conteúdo de mesmo nível |
| 5 | `panelBox(título, linhas, l, a, foco)` | 2 linhas + 2 colunas | área interativa, foco visível, conteúdo complexo |

Duas informações serem diferentes **não** é motivo para uma caixa.

```
sectionHeader("BRANCHES", 40, ColorWarning)
BRANCHES ───────────────────────────────
```

Outras peças da fundação:

- `panelTitle("RUNS", "8/40", "main")` → `RUNS · 8/40 · main`. Um formato só —
  antes existiam `NOME (12)`, `NOME · 12 linhas` e `NOME 3/5 PRONTOS` na mesma
  tela. Parte vazia é pulada.
- `keyHint("shift+P", "push")` — a aparência de tecla+efeito no app inteiro:
  tecla em destaque, descrição apagada. Rodapé, barra de comandos e trilho de
  ações usam a mesma.
- `truncateVisible` · `fitExactLines` · `clampRenderedHeight` — largura e altura
  exatas, contando colunas e nunca bytes.
- `glyphVocabulary` / `glyphBanned` — o vocabulário de glifos, travado por teste
  (§8.4).

Densidade: `projectTiny()` (altura < 22), `projectCompact()` (altura < 34 **ou**
largura < 110) e `dashboardCompact()` (altura < 28). Uma tela grande não é a
pequena esticada — cada faixa decide **o que cabe**, não só o quanto estica.

---

## 1.2 Navegação: módulos contextuais

A barra lateral lista **os módulos que fazem sentido para o projeto aberto**, não
os quinze que o DevScope sabe fazer. Num projeto Go sem Docker, sem Jenkins e
sem túnel, quinze linhas são treze coisas que não existem.

Cinco grupos, e cada rótulo responde a uma pergunta:

| grupo | pergunta | módulos |
|-------|----------|---------|
| `PROJETO` | o que é isto aqui? | Visão Geral |
| `CÓDIGO` | o que mudou, e o que roda em cima disso? | Git · GH Actions · Jenkins |
| `EXECUÇÃO` | o que está no ar? | Containers · Swarm · Kubernetes |
| `REDE` | por onde se chega? | Nginx · Rotas · Ngrok · SSH · CF Tunnel |
| `DADOS` | com o que eu falo? | API · Database · WebSocket |

A cor do grupo é a cor de destaque do módulo no app inteiro (`tabAccentColor`):
cabeçalho, borda do trilho, foco do painel. **Uma cor por categoria, não por
módulo.**

### Duas descobertas, dois custos

| | `probeToolLanding` (`landing_probe.go`) | `probeModuleCaps` (`module_caps.go`) |
|---|---|---|
| quando | ao **entrar** no módulo | ao **abrir** o projeto |
| custo | processo + rede (`kubectl config`, ping do ngrok) | `LookPath` + `Stat` |
| serve para | a landing dizer "kubectl não está instalado" | a sidebar saber **antes** de você ir lá |

A segunda é deliberadamente mais burra. Nenhum processo é criado, nenhuma rede é
tocada, e ela roda fora do `View` (§13).

### As duas regras que valem mais que a precisão

1. **Na dúvida, mostre.** Enquanto a sondagem não voltou, tudo aparece
   (`moduleCaps.relevant` devolve `true` com `!ready`). Módulo escondido por
   engano é funcionalidade perdida; módulo a mais é uma linha.

   **Toda porta de entrada tem de medir.** `openProject` sonda; `Init()` sonda
   quando o `devscope` abre dentro de um projeto (`openProjectFromCwd`), que
   entra direto no módulo sem passar por `openProject`. Esse caminho ficou sem
   sondagem e o "na dúvida, mostre" virou o modo permanente: os quinze módulos,
   em todo projeto, sempre.

   **E medir de novo quando o dado chega.** A sondagem lê `p.Git` e
   `p.Containers`, que a varredura preenche depois — medir antes classifica um
   repositório git como "sem git", e a medida errada não se corrige sozinha.
   `handleProjectGitLoaded` remede.
2. **Nada some de verdade.** O que ficou de fora vira `⋯ N módulos ocultos`, e a
   tecla `t` traz tudo de volta — apagado, para ensinar sem legenda por que não
   estava ali. A **aba ativa nunca some**, mesmo irrelevante.

Grupo que ficou sem módulo some junto: rótulo órfão é pior que a linha que ele
deveria titular.

`tab`/`shift+tab` andam só pelo que está **na tela** (`App.visibleTabs()`).
`AllTabs` é a lista completa e tem de estar na mesma ordem da sidebar — quando
divergem, o `tab` pula para uma linha que está acima na tela.

### Largura: a sidebar corta, não quebra

`lipgloss` **quebra** a linha que passa da caixa em vez de cortar, e cada linha
quebrada empurra o rodapé para fora do painel. Toda linha do trilho — status,
dica, hostname, régua de teclas — passa por `truncate`/`padRightVisible` na
largura real (18 colunas em modo compacto, não as 22 que estavam escritas à
mão).

---

## 1.3 Tela de lista + detalhe (Containers)

O padrão de toda tela que é **uma lista e o item sob o cursor**:

```
┌─LISTA · 7 · ↓4──────────────────────────────────┐   ← pede a altura que precisa
│ ⣴⣾⣦ svc      img:latest    3.1%   220M      2h  │
└─────────────────────────────────────────────────┘
──────────────────────────────────────────────────    ← detalhe começa numa régua
⣴⣾⣦ svc   running   img:latest                ⧗ 2h    identidade
  portas   :80 → 80/tcp · :443 → 443/tcp             ┐ factLine (§1.1)
  recursos CPU 12% ⣤⣦ · MEM 400M ⣴⣤ · NET 1.2MB     ┘
┌─LOGS · svc──────────────────────────────────────┐   ← a ÚNICA caixa do detalhe
│ ...                                             │
│ 2026/09/10 10:23:02 [error] upstream timed out  │   ← cauda encostada embaixo
└─────────────────────────────────────────────────┘
 enter portas · m detalhe · e shell · s parar …       ← barra de comandos
```

**A lista pede, o detalhe fica com o resto.** Divisão fixa em percentual deixa
dez linhas em branco dentro da lista quando há sete containers, e espreme os
logs em oito. A lista pede `n + moldura + cabeçalho`; o detalhe tem piso
(identidade + fatos + log legível) e leva **toda** a sobra — folga guardada como
linha em branco é o mesmo desperdício, só que do outro lado da tela.

**Um contexto, uma caixa.** Eram quatro molduras — LISTA, LOGS, STATS, PORTAS —
para falar de um container. Só o log ganha caixa: é o único que tem um dentro e
um fora (rola, e a moldura marca até onde). Identidade e fatos ficam sem
moldura, com o mesmo `factLine` do painel de projeto do dashboard.

**A cauda encosta embaixo.** Log preenche por cima (`padLinesTop`), como um
`tail -f`. Com o preenchimento embaixo a linha mais recente flutua no meio da
caixa e o olho a procura a cada atualização.

**Comparar antes de descrever.** Colunas que se *varre* (CPU, MEM, TEMPO)
sobrevivem à estreiteza; as que se *lê num item só* (IMAGEM, PORTAS) cedem
primeiro — elas estão no detalhe, logo abaixo.

**Contador no título, não em linha.** `LISTA · 7 · ↓4`. Um "↓ 4 abaixo" como
última linha é cortado pelo `fitExactLines` que fecha a caixa: o aviso nunca
chega à tela.

**Tecla de modo compra resolução.** `g` cicla a métrica; focado, o histórico
ocupa a largura toda e ganha média e pico (§5.1). Modo que só apaga linhas não
paga a tecla.

---

## 1.4 Gaveta: o que não é ambiente

Nem toda informação merece ocupar a tela o tempo todo. Antes de dar uma caixa
permanente a um painel, pergunte **o que ele é**:

| natureza | exemplo | onde vive |
|----------|---------|-----------|
| **ambiente** | branches, commits, worktree, lista de containers | painel fixo |
| **evento** | saída do `git pull`, resultado do `compose up` | gaveta que **abre sozinha** quando o evento acontece |
| **consulta** | stashes, imagens, dependências | gaveta atrás de uma tecla |

A tela principal do Git mostrava cinco caixas — BRANCHES, COMMITS, ALTERAÇÕES,
STASHES e LOG DE COMANDOS — para responder a três perguntas. As duas últimas não
são ambiente: a saída do `git pull` interessa nos dez segundos seguintes e depois
vira ruído fixo no rodapé; os stashes são consulta ocasional (e só apareciam
acima de 96 colunas).

Regras da gaveta (`internal/ui/git_drawer.go`):

1. **Uma faixa, vários conteúdos.** Um estado (`gitDrawer`), não uma caixa por
   assunto. Ela empresta espaço do corpo — nunca passa de metade dele, e some
   quando não há o que emprestar.
2. **Evento abre sozinho.** Terminou o comando, a gaveta do log abre com o foco
   dentro. Era o único valor da caixa permanente, e ela cobrava a tela por ele.
3. **A mesma tecla abre e fecha**, e `esc` fecha antes de sair do módulo.
4. **O foco não entra no que não está na tela.** `←→` só passa pelo log com a
   gaveta aberta; fechá-la devolve o foco a um painel visível.
5. **A tecla mora junto do dado que ela abre.** `3 stash s` na régua de
   contadores — a barra de comandos corta pelo fim, e a única porta para um
   recurso não pode ser a primeira a cair.

---

## 1.5 Faixa de abertura do módulo

Treze dos quinze módulos abrem numa tela de antes-de-entrar. Ela responde a
**três perguntas**: o que é isto, dá para usar aqui, como eu entro.

```
⇪ NGROK   ~/projetos/portfolio-main                    ⣴⣾⣦⡀ Running  ⣤ ok
                                            ← a faixa começa a 1/4 da altura


══════════════════════════════════════════════════════════════════════════  ← régua da cor do módulo
 ⇪  NGROK                                        ○ agente local offline     identidade | estado
    expõe o ambiente local — túnel por projeto, requests ao vivo
    enter abre o console; start sobe o agente                               nota (o que fazer)

    versão 3.37.3  ·  api :4040  ·  config .devscope/ngrok.json             fatos DEITADOS

    ▸ enter abrir console                                    esc voltar     ação | saída
──────────────────────────────────────────────────────────────────────────  ← fecha a faixa
```

Preencha `moduleLanding` em `internal/ui/module_landing.go`; o desenho é um só.

### Por que faixa, e não cartão

Duas tentativas erraram o mesmo alvo antes desta:

1. Tirar as quatro caixas e deixar cinco parágrafos soltos — **tirou a moldura
   em volta do vazio sem tirar o vazio**.
2. Prender os parágrafos num cartão e centrá-lo — **só mudou o vazio de lugar**.
   Sete linhas de conteúdo boiando numa tela de quarenta e quatro continuam
   parecendo tela quebrada.

O problema é de **proporção**, não de arranjo: esta tela tem pouco a dizer e
muita tela, e o que sobra é **largura**. Então o conteúdo **deita** em vez de
empilhar, e as **duas réguas dão fechamento** — o que vem depois lê como "a
página acaba aqui", não como "está faltando alguma coisa".

### A lista é o que faz a tela valer

Uma faixa de cinco linhas num painel de quarenta e cinco continua parecendo
tela quebrada, por melhor que esteja arranjada. O que faltava não era layout:
era **dado**.

Toda abertura mostra **o que ESTE projeto tem para o módulo operar** —
`previewTitle` + `preview` em `moduleLanding`:

| módulo | a lista é |
|---|---|
| Swarm · Jenkins | os containers do projeto (viram services / o que o build produz) |
| Kubernetes | os manifests achados em `k8s/`, `manifests/`, `deploy/` |
| GH Actions | os workflows de `.github/workflows` |
| Nginx | as rotas já cadastradas |
| Ngrok · CF · SSH · Rotas | as portas e domínios que o projeto publica |
| Database · API | os alvos detectados / o histórico |

Ela responde à única pergunta que existe antes do `enter`: **"o que eu vou
encontrar lá dentro?"**. Usa `sectionHeader` — degrau 3 da escada da §1.1, e a
razão de a escada existir —, cresce até ocupar o vão do painel e corta com `+N`.

A sondagem já montava essas listas e **guardava só a contagem**: `ghaProcs`,
`k8sManifests`, `nginxCount` jogavam fora exatamente o que a tela tinha para
mostrar.

### Regras

1. **O estado mora na faixa**, não repetido no canto do cabeçalho (§1: uma
   informação, um lugar). É a resposta a "dá para usar isto aqui?".
2. **Estado diz o ESTADO; o que fazer é nota.** `○ agente local offline` na
   linha da identidade; `enter abre o console` na linha de baixo. Estado que
   explica a tecla disputa espaço com o nome do módulo e some em 70 colunas.
3. **Sondagem pendente diz "sondando o ambiente…"** — nunca "offline" antes de
   medir (`landingProbing`, `landingToolState`).
4. **Fatos deitam quando cabem** e empilham alinhados quando não
   (`landingFactRows`). Três pares curtos em três linhas fazem uma coluna
   esburacada e gastam três linhas para nove palavras.
5. **A coluna do valor segue os rótulos que existem**, não o teto global de
   `factLabelW`: rótulos curtos ("lê", "gh") abriam buraco.
6. **A primeira ação leva `▸` e negrito**; `esc` é saída, não ação, e vai
   encostado na direita.
7. **Fato que só repete o estado é ruído.** Sem a ferramenta no PATH, "cli não"
   já foi dito no aviso acima.
8. **Nada de folheto.** Lista do que o módulo *sabe fazer* fala do DevScope, não
   deste projeto. Fato que não muda com o projeto aberto é tagline — uma linha —
   ou não é nada.
9. **A faixa é ancorada no topo**, uma linha abaixo do cabeçalho. Ela é o
   conteúdo da tela, não um aviso flutuando.
10. **`countOf` para contagem**: "1 container", não "1 containers".
11. **Pulso de saudável é `okPulse()`**, nunca `a.pulse()` — esta começa em `⣀`,
    o mesmo glifo de "parado", e em screenshot parado os dois viravam a mesma
    coisa (§6).

Módulo com conteúdo real na abertura (Logs) **não tem faixa**: segue o padrão de
lista+detalhe (§1.3).

---

## 1.6 Responsividade: regras de degradação

Cinco tamanhos, e o DevScope serve os cinco:

| | |
|---|---|
| `80×24` | o mínimo do POSIX — e o terminal de split do VS Code |
| `100×30` | metade de um monitor 1080p |
| `120×40` | a janela típica em tela cheia |
| `160×50` | ultrawide ou fonte pequena |
| `200×60` | duas telas, ou fonte muito pequena |

`internal/ui/responsive_test.go` renderiza **toda tela nos cinco** e cobra as
invariantes abaixo. Tela nova entra em `responsiveScenes()` ou não é auditada.

### As três classes de tela

A regra "tela grande não é a pequena esticada" **não vale igual para toda
tela**, e fingir que vale produz preenchimento inventado — que o §1 proíbe.

| classe | o que é | é cobrada por crescer? |
|---|---|---|
| **rolável** | tem mais conteúdo do que cabe: dashboard, listas, logs, diff | **sim** |
| **portão** | diz o estado e a tecla, e acabou: a abertura de módulo (§1.5) | não — cresce com a lista de preview até onde o projeto tem itens |
| **prompt** | uma pergunta e um campo: modal de tema, busca, confirmação | não — a altura extra vira moldura |

Estar na lista `boundedScenes` é **decisão de design registrada**, com o motivo
escrito ao lado — não isenção de teste: elas continuam cobradas por não estourar
e por não sumir com conteúdo em tela grande.

### Onde o vazio pode ficar

O vazio pode existir — nem toda tela tem o que dizer. Mas fica **num lugar só,
no fim**. Vazio no meio lê como layout quebrado; o mesmo vazio embaixo lê como
página que terminou.

Foi o defeito da Dashboard em 200×60: o painel do selecionado ficava ancorado no
rodapé e abria **vinte linhas de buraco** entre ele e a lista. Agora ele sobe
para junto da lista e a sobra fica inteira acima da barra de comandos.

O último buraco é legítimo — a barra de comandos é ancorada por design (§2).

### Escada de expansão (Dashboard)

Altura extra vira **detalhe do projeto sob o cursor**, não vão:

| altura | o painel ganha |
|---|---|
| `< 32` | nada — a lista leva tudo |
| `>= 32` | os três fatos: stack · runtime · git |
| `>= 44` | + os containers do projeto, um por linha |
| `>= 54` | + as probes de saúde |

A lista tem prioridade: pega a altura primeiro, e o painel cresce com o que
sobrar. Overlay (ajuda, tema) tem a altura do que mostra — nunca o teto da tela.

### Ordem de sacrifício das colunas

**Comparar antes de descrever.** O que se varre com o olho sobrevive; o que se
lê num item só cede primeiro, porque já está no detalhe logo abaixo.

| tabela | sempre | cede primeiro |
|---|---|---|
| projetos | glifo · NOME · BRANCH · CAMINHO | — (as opcionais já saíram) |
| containers | glifo · NOME · CPU · MEM · TEMPO | PORTAS, depois IMAGEM |

A ordem é **monotônica**: coluna que entrou numa largura não some numa maior.

### As dicas encurtam, não mudam de palavra

A barra de status **descarta do fim**; ela não reescreve. Antes trocar de
terminal trocava o nome do atalho: `pgup/pgdown rolar` virava `↑↓/pg scroll`,
`tab/shift+tab módulo` virava `tab módulo`, `q sair` virava `? help`. Eram cento
e trinta linhas com a mesma cadeia de catorze `if` escrita duas vezes.

Agora é uma lista ordenada por importância em `App.projectHints`, e o que se lê
em 200 colunas é **o começo** do que se lê em 80.

---

## 2. Anatomia da tela padrão

Toda tela de módulo tem quatro faixas, nesta ordem:

```
▣ MÓDULO   identidade   ⣷⣄⣀⣀⣴ estado            contexto  ·  contexto  ·  hh:mm:ss
 1 ABA │ 2 ABA │ 3 ABA │ 4 ABA │ 5 ABA                              indicador de estado
┌─TÍTULO · 40 linhas · 12 erros────────────────────────────────────────────────────┐
│  1 │ conteúdo                                                                    │
│  2 │ conteúdo                                                                    │
└──────────────────────────────────────────────────────────────────────────────────┘

 1-5 abas · ↑↓ rolar · ←→ lateral · / buscar · r recarregar · esc voltar
```

1. **Cabeçalho de identificação** — quem é, como está, onde escuta. Uma linha.
2. **Régua de abas numerada** — à esquerda as abas, à direita o indicador de
   estado (posição, busca, "ao vivo").
3. **Corpo** — painéis. Ocupa toda a altura restante.
4. **Barra de comandos** — larga, no rodapé, presa embaixo.

Esqueleto (referência: `internal/ui/container_detail_views.go`):

```go
func (a *App) renderModuleChrome(body string) string {
	w := maxInt(40, a.width)
	h := maxInt(12, a.height-1)

	header := a.renderModuleHeader(w)
	tabs := a.renderModuleTabBar(w)
	cmdBar := a.renderModuleCommandBar(w)

	stack := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	return clampRenderedHeight(lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar), h)
}
```

O preenchimento (`fill`) é obrigatório: sem ele a barra de comandos sobe e cola
no conteúdo quando ele é curto.

### 2.1 Cabeçalho

`glifo MÓDULO` em cor de destaque + identidade em negrito + onda de status + a
direita, os dados de contexto separados por `  ·  ` e o relógio.

```go
left := accent.Render("▣ CONTAINER") + StyleMuted.Render("   ") +
	StyleNormal.Bold(true).Render(truncate(name, 28))
left += "   " + waveStyle.Render(wave) + " " + labelStyle.Render(label)
return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
```

Caminho longo corta **pela esquerda** (`elideLeft`): o prefixo se repete em toda
linha, a cauda é o que identifica.

### 2.2 Régua de abas

Numerada, sempre. Sem o número não há como saber que dá para pular direto para a
aba 4.

```go
label := fmt.Sprintf(" %d %s ", i+1, strings.ToUpper(tab.shortLabel()))
if compact { // width < 78: só o número, exceto a ativa
	label = fmt.Sprintf(" %d ", i+1)
}
```

Separador `│` em `StyleMuted`, aba ativa em `StyleSelected`. A aba ativa **nunca**
é truncada, mesmo no modo compacto.

### 2.3 Barra de comandos

Substitui a coluna vertical "AÇÕES", que espremia 17–20 atalhos truncados em
17 colunas e roubava largura do conteúdo.

```go
return StyleStatusBar.Width(width).Render(fitKeybindsWrap(maxInt(10, width-2), 2, items...))
```

- `fitKeybindsWrap(width, maxLines, items...)` quebra em até 2 linhas e descarta
  o que não couber, começando pelo fim da lista — ordene por importância.
- Os itens mudam com o contexto: `n/N ocorrência` só aparece com busca ativa,
  `f acompanhar` só na aba de logs, `0 voltar ao início` só quando há rolagem
  lateral.
- **Não anuncie tecla que não existe.** A régua dizia `1-7 abas` meses antes de
  os números serem ligados a alguma coisa.

---

## 3. Teclado

### As três camadas

O DevScope trata **mais de mil bindings em setenta e seis handlers**. Nesse
tamanho, "toda ação tem uma forma clara de descoberta" só é verdade se for
verificável — senão um módulo novo nasce com teclas que só quem escreveu
conhece. Foi o caso de JSON e JWT: dois módulos inteiros sem bloco na ajuda.

| camada | onde vale | exemplos |
|---|---|---|
| **GLOBAL** | qualquer tela | `?` ajuda · `q` sair · `esc` voltar |
| **CONTEXTUAL** | a tela atual | `t` módulos · `/` filtrar · `r` atualizar |
| **LOCAL** | o módulo aberto | `s` é stash no Git e parar no ngrok |

**LOCAL vence CONTEXTUAL vence GLOBAL** — o handler do módulo aberto roda antes.
É por isso que `t` abre a lista de módulos na tela de projeto e testa a conexão
dentro do Jenkins, sem conflito.

### O que `keymap_test.go` cobra

1. **Toda tecla tratada está na ajuda**, é navegação, ou tem o motivo registrado
   em `keysWithoutHelp` — decisão escrita, não esquecimento silencioso.
2. **A ajuda nunca promete tecla que ninguém trata.** A régua já anunciou
   `1-7 abas` meses antes de os números fazerem alguma coisa (§2.3).
3. **Todo módulo da sidebar tem bloco na ajuda.** Módulo sem bloco nasce mudo.
4. **Uma ação, um nome, em português.** `refresh`, `atualizar`, `recarregar` e
   `reescanear` eram quatro palavras para recarregar, em módulos diferentes.
5. **Movimento tem um nome só.** `pgup/pgdown` é "rolar" em toda tela; o que
   muda por contexto é o alvo, não a tecla.

Duas armadilhas que o teste já pegou:

- O handler GLOBAL usa `case msg.String() == "x":`, e não `case "x":`. Um
  extrator que só vê a segunda forma fica cego para as teclas que valem em toda
  tela — que são justamente as mais importantes.
- **Tecla se escreve por inteiro.** `ctrl+c · x · v` parece econômico e é
  ambíguo: `x` sozinho não é `ctrl+x`.

| Tecla | Função | Observação |
|-------|--------|------------|
| `1`…`9` | trocar de aba | única forma de trocar de aba |
| `↑↓` / `k` `j` | rolar vertical | |
| `←→` / `h` `l` | rolar lateral | 8 colunas |
| `shift+←→` / `H` `L` | rolar lateral | meia tela |
| `0` | voltar ao início da linha | |
| `pgup` / `pgdown` | página | também `shift+↑↓` |
| `home` `g` / `end` `G` | topo / fim | |
| `/` | buscar | `n` próxima, `N` anterior |
| `r` | recarregar | |
| `esc` | voltar um nível | limpa a busca antes de sair |

Regras:

- **Seta nunca troca de aba.** Seta é rolagem; aba é número.
- Modo de digitação (busca, filtro, formulário) intercepta **antes** do handler
  da tela, senão digitar "4" troca de aba no meio de uma busca.
- `Shift+T` (tema) só existe na tela inicial, onde não há campo de texto.
- Atalho novo entra na barra de comandos no mesmo commit em que é ligado.

---

## 4. Corpo da tela

### 4.1 Painel único, largura cheia

Conteúdo textual (log, config, YAML, saída de comando) vai num painel só, com
numeração de linha:

```go
num := StyleMuted.Render(padLeft(strconv.Itoa(i+1), gutter))
line := num + StyleMuted.Render(" │ ") + a.renderCodeLine(all[i], textW, matched, current)
```

- Gutter mínimo de 3 colunas. Numerar é o que permite falar sobre a linha.
- A largura do texto sai de **uma função só**, usada pelo render e pelo passo de
  rolagem lateral — senão a rolagem passa do fim do texto:

```go
func (a *App) containerDetailTextWidth() int {
	inner := maxInt(20, maxInt(40, a.width)-2)
	gutter := maxInt(3, len(strconv.Itoa(a.containerDetailContentLen())))
	return maxInt(8, inner-gutter-3) // gutter + " │ "
}
```

### 4.2 Organizar antes de realçar

O conteúdo bruto do sistema chega desalinhado. Organize primeiro
(`container_detail_layout.go`), realce depois:

| Formato | Tratamento |
|---------|------------|
| Tabela (`docker top`, `ps`) | colunas alinhadas pelo cabeçalho; última coluna livre |
| `CHAVE=valor` | valores alinhados numa coluna, separador colado à chave |
| `Chave: valor` em blocos | alinhamento por nível de indentação, com teto de padding |
| YAML / compose | chave em destaque, `:` e `- ` apagados, `#` inteiro apagado |
| Dockerfile | instrução (`FROM`, `RUN`, `COPY`…) em destaque |
| Log | carimbo apagado, nível colorido, mensagem normal |
| JSON | reaproveitar `jsonKindsForRunes` + `styleJSONRune` |

Duas regras invioláveis:

1. **Realce só pinta.** Fora o alinhamento, o texto visível é idêntico ao que o
   sistema escreveu — é ele que vai colado num chamado.
2. **Valor que parece segredo é mascarado** (`SECRET`, `PASSWORD`, `TOKEN`,
   `APP_KEY`, `_KEY`, `SALT`, `DSN`…): duas primeiras letras + `•`. Esta tela é
   mostrada em call e em print.

### 4.3 Título do painel

`TÍTULO · 40 linhas · 12 erros` — contadores no título, separados por `·`,
**omitindo o que for zero**. Nada de "0 avisos".

### 4.4 Truncamento lateral

Linha que continua à direita termina em `…`, mesmo já rolada (`sliceColumns`).
Sem isso não dá para saber se o fim da tela é o fim da linha.

---

## 5. Números: régua, não cards

**Nunca** um card por número. Seis cards de uma linha cada comem 5 linhas de
altura para mostrar 6 inteiros.

```
CPU   4.20% ⣤⣦⣶⣷  ·  MEM   3.40% ⣀⣄  ·  REDE 2 KB  ·  BLOCO 0 KB  ·  PIDS 14        20 amostras
```

- Uma linha, células separadas por `  ·  `, contexto à direita com
  `joinWithSpacer`.
- Fundir com a linha de navegação quando existir uma.
- Card (`renderStatsCard`) só quando o valor vem com **medidor** e ocupa a caixa
  inteira — não para um inteiro solto.

### 5.1 Gráficos

- Histograma preenche a largura da caixa: são no máximo 40 amostras e a caixa
  passa de 60 colunas, então **estique** (`hist[c*len(hist)/width]`), não desenhe
  uma coluna por amostra.
- O teto acompanha a janela em degraus fixos (10 / 25 / 50 / 100) e **vai escrito
  no título** (`CPU % · 0-25%`). Teto fixo em 100 desenha 4% como uma linha no
  fundo de 20 vazias; teto flutuante sem rótulo mente sobre a escala.
- Resumo embaixo: `agora · média · pico`.
- Não desenhe a mesma série duas vezes (sparkline em cima do histograma dela).

---

## 6. Vocabulário visual: Braille

Braille é o padrão do projeto para **estado, progresso e histórico**. Todo status
anima, subindo e descendo.

### 6.1 Ondas de status (`internal/ui/spark.go`)

`statusWave(kind, cols, frame)` desenha `cols` **colunas de ponto** (2 por
caractere; ímpar deixa a última metade vazia).

| Estado | Onda | Cor |
|--------|------|-----|
| `running` | senoide viajando, altura 1–4 | verde (`StyleRunning`) |
| `starting` | preenche da esquerda para a direita, nunca vazia | amarelo |
| `stopping` | esvazia da direita para a esquerda | amarelo |
| `restarting` | pico viajando sobre base 1 | amarelo (`StyleWarning`) |
| `unhealthy` | uniforme, respira 2↔3 | **amarelo**, não vermelho |
| `paused` | uniforme no topo (4) | amarelo |
| `exited` / `created` | uniforme baixa (1) | vermelho / cinza |
| `idle` | pontos alternados | cinza |
| `always` (política) | onda do estado real | **azul** (só a onda muda de cor) |

Larguras adotadas: **10 colunas** na lista de containers
(`containerWaveCols`), **7 colunas** na tela inicial (`projectWaveCols`).

Dois estados **não podem** colidir no mesmo glifo em nenhum quadro — existe teste
para isso.

### 6.2 Pulso inline (`internal/ui/anim.go`)

`pulseGlyph(level, frame)` devolve **um caractere**, para chips e rótulos
(`⣷ ao vivo`). Famílias disjuntas: `pulseOK` chega ao topo (`⣿`), `pulseWarn` usa
só a coluna esquerda, `pulseBad` fica na base, `pulseIdle` treme.

### 6.3 Histórico

- `brailleSpark(samples, cells)` — 2 amostras por célula, alinhado à direita.
- `meterBar(pct, width)` — medidor com cor por faixa.
- `renderMetricSparkline(hist, width, maxHint)` — `maxHint <= 0` normaliza pelo
  pico da janela.

### 6.4 Carregamento

Loading é braille (`a.loadingText("carregando logs…")`). Não use spinner ASCII
nem emoji.

### 6.5 Animação

`animInterval = 100ms` (10 fps). O tick só roda quando a tela precisa
(`needsAnim` / `wantsPulseAnim`) — TUI parada não deve acordar a CPU.

---

## 7. Cores e tema

- **Nunca hardcode hex.** Use os tokens (`ColorAccent`, `ColorDanger`, …) e os
  estilos (`StyleNormal`, `StyleMuted`, `StyleHealthy`, `StyleWarning`,
  `StyleUnhealthy`, `StyleSelected`, `StyleKey`, `StyleSection`, …). São 12 temas;
  hex fixo quebra 11 deles.
- Cor de destaque do módulo: `tabAccentColor(TabX)`.
- Semântica: verde = saudável, amarelo = atenção/transição, vermelho = parado ou
  quebrado, cinza = inexistente/ocioso, azul = informação e política.
- Registro que **não pertence ao projeto aberto** (visão `Shift+A`) mostra o nome
  do projeto em amarelo. Órfão (`ProjectPath == ""`) também conta como de fora.

---

## 8. Largura, altura e glifos

O maior gerador de bug visual neste projeto.

1. **Largura de coluna sai da largura do painel, não de `a.width`.** Calcular com
   `a.width` e renderizar dentro de um painel menor trunca duas vezes
   (`Merge branc…`, `endpoint_…`).
2. **Truncar com consciência de ANSI.** Use `truncateVisible`, `padRightVisible`,
   `ansi.Truncate` — nunca corte por byte ou por rune uma string já estilizada.
3. **Coluna de largura zero não emite separador.** Use
   `joinNonEmpty(sep, parts...)`, senão o cabeçalho estoura em 1 coluna.
4. **Evite glifo de largura ambígua.** `⚡ ☰ 🔀 🔒 📄 🍒` são medidos como 2
   colunas pelas libs e desenhados como 1 pela maioria dos terminais — a linha
   inteira sai desalinhada. Substituídos por `⇅ ≡ ⑂ ⚿ ⊕`. Braille e caixa
   (`─ │ ┌ ╭ █`) são seguros.
   O vocabulário permitido está em `glyphVocabulary` e o banido em `glyphBanned`
   (`foundation.go`). `foundation_test.go` varre o código-fonte: glifo banido não
   volta por um módulo novo, e glifo novo só entra se medir 1 coluna nas **duas**
   libs (`go-runewidth` e `lipgloss`) — onde elas discordam, a linha sai alinhada
   num terminal e torta no outro.
5. **Conte o prefixo do cursor** ao dimensionar a coluna da direita.
6. `fitExactLines(lines, height)` para preencher a caixa e
   `clampRenderedHeight(content, height)` para nunca passar da tela.

---

## 9. Estados

| Estado | Regra |
|--------|-------|
| **Carregando** | `a.loadingText("carregando <coisa>…")`, dentro da caixa que vai receber o dado |
| **Vazio** | Diz o motivo **e** a saída: `"sem saída ainda — r recarrega, f acompanha ao vivo"` |
| **Erro** | Uma linha em `StyleWarning` com o que falhou, e o que fazer. Sem stack trace |
| **Sem amostra útil** | Faixa de aviso, não uma tela em branco |
| **Sem ferramenta** | Degradar: se não há docker, o resto da tela continua funcionando |

---

## 10. Microcópia

- **Português**, minúsculas nas dicas, CAIXA ALTA nos títulos de painel e abas.
- Verbo no infinitivo nos atalhos: `recarregar`, `voltar`, `acompanhar`.
- Diga o efeito, não o comando: `↑2 p/ enviar`, não `ahead 2`.
- Tempo em forma curta (`2d 4h`, `12m`), sem `0m` sobrando depois dos dias.
- Ausência é `—` (`emDash`), nunca `n/a`, `null` ou vazio.

---

## 10.1 Um nome, uma grafia

Duas doenças da mesma família, e as duas apareceram só quando o app ficou grande
o bastante para ninguém ver as duas telas juntas:

| | tinha | tem |
|---|---|---|
| **estado** | `Running` na sidebar, `rodando` no dashboard | `projectStatusWord` é a única fonte |
| **tecla** | `S-R` na barra, `shift+R` na ajuda | `shift+R` nas duas |
| **verbo** | `refresh` · `atualizar` · `recarregar` · `reescanear` | `atualizar` |

Quem lê a ajuda e volta para a barra tem de reconhecer o atalho. `keymap_test.go`
cobra as três.

**Tecla se escreve por inteiro.** `ctrl+c · x · v` parece econômico e é ambíguo:
`x` sozinho não é `ctrl+x`.

---

## 11. Anti-padrões

| Anti-padrão | O que fazer |
|-------------|-------------|
| Card KPI com um número | Régua de contadores (§5) |
| Coluna vertical "AÇÕES" | Barra de comandos larga (§2.3) |
| Mesmo dado em 3 lugares | Escolha um; apague os outros |
| Moldura arredondada envolvendo a tela toda | Sem moldura externa |
| Painel "INSPECT" que só repete atalhos | Apagar |
| Conteúdo de exemplo / "em breve" | Não entra |
| Emoji como ícone | Glifo de 1 coluna (§8.4) |
| Aba trocada por seta | Número (§3) |
| Caixa só porque duas informações são diferentes | `sectionHeader` (§1.1) |
| `NOME (12)` no título do painel | `panelTitle("NOME", "12")` (§1.1) |
| Filete escrito à mão (`strings.Repeat("─", n)`) | `rule(n)` (§1.1) |
| Rodapé repetindo a barra de comandos | Um só |
| `viewport` calculado de dois jeitos | Uma função, usada por todos |
| Última linha vazia por causa do `\n` final | `strings.TrimRight(content, "\n")` |
| Devolver `" "` quando não há o que dizer | devolver `""` **e omitir a linha** — vazia ainda é linha |
| Sparkline sem amostra desenhada como faixa em branco | `—`; faixa tem a largura do que foi **medido** |
| Caixa esticada até o fim do painel com duas linhas dentro | altura do conteúdo, com piso e teto |
| Relógio ou contador mudando no canto de toda tela | movimento em área crítica; só onde a frescura decide algo |
| Coluna dimensionada por fração da largura | dimensionada pelo conteúdo **mais longo**, com teto |

---

## 12. Testes obrigatórios

Toda tela nova leva, no mínimo, as invariantes que já quebraram antes:

```go
// 1. Nada estoura a largura, em nenhum terminal
for _, w := range []int{60, 80, 100, 120, 160, 200} { ... lipgloss.Width(row) <= tableW-1 }

// 2. A tela preenche a altura do terminal
lipgloss.Height(got) >= a.height-2

// 3. Estados são distinguíveis sem cor, em todo quadro
for f := 0; f < 120; f++ { glifo(estadoA, f) != glifo(estadoB, f) }

// 4. Realce não altera o texto visível
stripANSI(highlight(line)) == line

// 5. Segredo não chega à tela
!strings.Contains(render, valorDoSegredo)

// 6. A tecla faz o que a barra de comandos promete
a.handleKeys(tea.KeyMsg{Type: tea.KeyRight}, p) // rola, não troca de aba
```

Use `stripANSI` para comparar conteúdo e `lipgloss.Width` para medir — nunca
`len()`.

---

## 13. Fluxo de trabalho

1. **Preview antes de commitar.** Um `zz_preview_test.go` temporário, protegido
   por `PREVIEW=<dir>`, que renderiza `a.View()` com dados realistas em 120×34 e
   140×40 e grava em arquivo. Olhe o arquivo; **apague o teste** antes de
   terminar.
2. **Dados realistas.** Preview com dado inventado esconde o problema real —
   use a saída de verdade do coletor (nginx com log de acesso *e* de erro,
   caminho longo, container sem porta).
3. **Fumaça no binário**, num pty de tamanho fixo:
   `script -q -c "stty rows 40 cols 150; devscope"`.
   Num pty cru é preciso responder às consultas do terminal (`OSC 11`, `CSI 6n`),
   senão o processo trava na inicialização.
4. `go build ./... && go vet ./... && go test ./... -count=1` antes de entregar.
5. `gofmt -w` **só nos arquivos que você tocou** — o repositório tem arquivos
   não formatados de antes.

---

## 14. Checklist antes de entregar uma tela

- [ ] Cabeçalho, régua numerada, corpo, barra de comandos — nesta ordem
- [ ] Sem moldura externa; a barra de comandos está presa embaixo
- [ ] Nenhum card de um número; contadores numa régua
- [ ] Todo atalho da barra existe; todo atalho que existe está na barra
- [ ] Número troca aba, seta rola
- [ ] Estados usam braille animado, com glifos disjuntos e cor por semântica
- [ ] Nenhum hex; só tokens de tema
- [ ] Testado em 60, 80, 120 e 200 colunas sem estouro nem truncamento duplo
- [ ] Estado vazio, carregando e erro escritos
- [ ] Segredo mascarado
- [ ] Preview apagado, testes passando

---

## 15. Fora do padrão

Os **modais** são um padrão à parte e não mudam junto com a tela que os abre:
os de Git (criar branch, commit, rename, delete), o assistente de nova rota do
Nginx (`tunnelModalBox`) e as confirmações de exclusão
(`renderTunnelDeleteConfirmBox`). Eles já são consistentes entre si — mexer em
um só quebra essa consistência.

A **landing de módulo** foi convertida em 2026-09-10 (§1.5): as treze passaram
juntas, que era a condição para não deixar uma diferente das outras. O trilho
direito `DETALHES`+`AÇÕES` (`renderModuleRightRail`) sobrevive só dentro do
console do Jenkins — converter é trabalho da tela dele, não da landing.

Nada mais está fora. O Nginx era exceção por pedido do dono do projeto e foi
convertido em 2026-09-08.
