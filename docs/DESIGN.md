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
| Rodapé repetindo a barra de comandos | Um só |
| `viewport` calculado de dois jeitos | Uma função, usada por todos |
| Última linha vazia por causa do `\n` final | `strings.TrimRight(content, "\n")` |

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

A **tela de abertura de cada módulo** (a landing com `renderModuleContext` +
trilho direito) é compartilhada pelos onze módulos. Ela tem o padrão dela;
converter a landing de um módulo isolado a deixaria diferente das outras dez.

Nada mais está fora. O Nginx era exceção por pedido do dono do projeto e foi
convertido em 2026-09-08.
