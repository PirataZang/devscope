package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

// ─── auditoria do teclado ───────────────────────────────────────────────────
//
// O DevScope trata mais de mil bindings em setenta e seis handlers. Nesse
// tamanho, "toda ação importante tem uma forma clara de descoberta" só é
// verdade se for verificável — senão um módulo novo nasce com teclas que só
// quem escreveu conhece. Foi o que aconteceu com JSON e JWT: dois módulos
// inteiros sem um bloco na ajuda.
//
// As três camadas do teclado:
//
//	GLOBAL      vale em qualquer tela: ? q esc tab ↑↓ pgup/pgdown
//	CONTEXTUAL  vale na tela: t (módulos), / (filtrar), r (atualizar)
//	LOCAL       vale no módulo aberto: s (stash no Git, stop no ngrok)
//
// LOCAL vence CONTEXTUAL vence GLOBAL — o handler do módulo aberto roda antes.

// keyNav são as teclas de NAVEGAÇÃO: o vocabulário que se aprende uma vez e
// vale em toda tela. Não precisam de bloco por módulo na ajuda; o bloco
// NAVEGAÇÃO cobre todas.
var keyNav = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"j": true, "k": true, "h": true, "l": true,
	"enter": true, "esc": true, "tab": true, "shift+tab": true,
	"pgup": true, "pgdown": true, "home": true, "end": true, "g": true,
	"backspace": true, "delete": true, " ": true, "space": true, "0": true,
	"shift+up": true, "shift+down": true, "shift+j": true, "shift+k": true,
	"shift+left": true, "shift+right": true, "shift+h": true, "shift+l": true,
	"ctrl+c": true, "q": true, "?": true, "/": true, "n": true, "p": true,
	"y": true, "Y": true, "N": true, "P": true, // confirmações e n/N de busca
}

// keysWithoutHelp são as teclas deliberadamente fora da ajuda, cada uma com o
// motivo. Estar aqui é decisão registrada, não isenção silenciosa — que é a
// diferença entre uma exceção e um esquecimento.
var keysWithoutHelp = map[string]string{
	"ctrl+u": "limpa o campo dentro do formulário, que já mostra as próprias teclas",
	"ctrl+j": "quebra de linha no compositor, ao lado do enter que envia",
	"ctrl+m": "alias de enter que alguns terminais mandam",
	"w":      "join-token de worker no formulário do Swarm, escrito no próprio formulário",
	"m":      "join-token de manager no formulário do Swarm, idem",
	"merge":  "nome de modo do prompt de git, não é tecla",
	"!":      "aviso de setup do GitHub CLI — anunciado na barra da própria tela",
}

// canonKey põe uma tecla na forma canônica — e expande as que representam
// mais de uma ("ctrl+←→" são duas). Os DOIS lados da comparação passam por
// aqui: normalizar só um lado é o que faz um teste de paridade dar falso
// positivo em "shift+a" contra "shift+A".
func canonKey(k string) []string {
	k = strings.TrimSpace(k)
	if k == "" {
		return nil
	}
	arrows := strings.NewReplacer("↑", "up", "↓", "down", "←", "left", "→", "right")
	// Um rótulo como "ctrl+←→" ou "shift+↑↓" vira duas teclas.
	for _, pair := range []struct{ glyphs, a, b string }{
		{"←→", "left", "right"}, {"↑↓", "up", "down"},
	} {
		if i := strings.Index(k, pair.glyphs); i >= 0 {
			pre := k[:i]
			return []string{strings.ToLower(pre + pair.a), strings.ToLower(pre + pair.b)}
		}
	}
	k = strings.ToLower(arrows.Replace(k))
	out := []string{k}
	// Terminais mandam Shift+A ora como "shift+a", ora como "A": as duas formas
	// são a mesma tecla para quem lê a ajuda.
	if base, ok := strings.CutPrefix(k, "shift+"); ok {
		out = append(out, base)
	} else if len(k) == 1 && k >= "a" && k <= "z" {
		out = append(out, "shift+"+k)
	}
	return out
}

var keyCaseRe = regexp.MustCompile(`case ((?:"[^"]*"(?:, )?)+):`)

// keyGlobalRe pega a outra forma: `case msg.String() == "ctrl+t":`. É como o
// handler GLOBAL é escrito, e sem ela a auditoria ficava cega exatamente para
// as teclas que valem em toda tela.
var keyGlobalRe = regexp.MustCompile(`msg\.String\(\) == "([^"]+)"|key\.WithKeys\(((?:"[^"]*"(?:, )?)+)\)`)
var keyFuncRe = regexp.MustCompile(`^func \(a \*App\) (handle\w*Keys|update\w+)\(`)

// handledKeys varre o código e devolve toda tecla tratada DENTRO de um handler
// de teclado — `case "running"` num switch de status não conta.
func handledKeys(t *testing.T) map[string][]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		inHandler := false
		for _, line := range strings.Split(string(src), "\n") {
			if keyFuncRe.MatchString(line) {
				inHandler = true
				continue
			}
			if line == "}" {
				inHandler = false
				continue
			}
			if !inHandler {
				continue
			}
			for _, m := range keyCaseRe.FindAllStringSubmatch(line, -1) {
				for _, k := range strings.Split(m[1], ", ") {
					for _, c := range canonKey(strings.Trim(k, `"`)) {
						out[c] = append(out[c], f)
					}
				}
			}
			for _, m := range keyGlobalRe.FindAllStringSubmatch(line, -1) {
				for _, group := range m[1:] {
					for _, k := range strings.Split(group, ", ") {
						for _, c := range canonKey(strings.Trim(k, `"`)) {
							out[c] = append(out[c], f)
						}
					}
				}
			}
		}
	}
	return out
}

// helpKeys expande os rótulos da ajuda no conjunto de teclas que eles prometem:
// "1-3" são três teclas, "p · m" são duas, "onde" é prosa.
func helpKeys() map[string]bool {
	prose := map[string]bool{
		"Commands": true, "name": true, "command": true, "arquivo": true,
		"cli": true, "digitar": true, "onde": true, "file": true,
		"GLOBAL": true, "CONTEXTUAL": true, "LOCAL": true, "quem vence": true,
	}
	out := map[string]bool{}
	add := func(k string) {
		for _, c := range canonKey(k) {
			out[c] = true
		}
	}
	for _, g := range helpGroups() {
		for _, b := range g.blocks {
			for _, e := range b.entries {
				if prose[strings.TrimSpace(e.keys)] {
					continue
				}
				for _, part := range strings.Split(e.keys, "·") {
					part = strings.TrimSpace(part)
					if a, b, ok := strings.Cut(part, "-"); ok && len(a) == 1 && len(b) == 1 &&
						a[0] >= '0' && a[0] <= '9' && b[0] >= '0' && b[0] <= '9' {
						for d := a[0]; d <= b[0]; d++ {
							add(string(d))
						}
						continue
					}
					for _, tok := range strings.Fields(part) {
						add(tok)
					}
				}
			}
		}
	}
	return out
}

// TestEveryActionIsDiscoverable: toda tecla que o app trata está na ajuda, é
// navegação, ou tem motivo registrado para não estar.
//
// Foi este teste que expôs JSON e JWT sem bloco nenhum — dois módulos cujas
// teclas só existiam dentro de uma caixa "ATALHOS" que a sessão 6 removeu.
func TestEveryActionIsDiscoverable(t *testing.T) {
	help := helpKeys()
	var orphans []string
	for key, files := range handledKeys(t) {
		if keyNav[key] || help[key] {
			continue
		}
		if _, ok := keysWithoutHelp[key]; ok {
			continue
		}
		// Dígito solto é aba numerada; o bloco do módulo já diz "1-N abas".
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			continue
		}
		sort.Strings(files)
		orphans = append(orphans, key+"  ("+strings.Join(uniqueStrings(files), " ")+")")
	}
	sort.Strings(orphans)
	if len(orphans) > 0 {
		t.Errorf("%d tecla(s) sem forma de descoberta — ponha na ajuda ou registre o motivo em keysWithoutHelp:\n  %s",
			len(orphans), strings.Join(orphans, "\n  "))
	}
}

// TestHelpNeverPromisesAKeyThatDoesNotExist: o contrário do teste acima. A
// régua do DevScope já anunciou "1-7 abas" meses antes de os números fazerem
// alguma coisa (docs/DESIGN.md §2.3).
func TestHelpNeverPromisesAKeyThatDoesNotExist(t *testing.T) {
	handled := handledKeys(t)
	has := func(k string) bool {
		for _, c := range canonKey(k) {
			if _, ok := handled[c]; ok {
				return true
			}
		}
		return false
	}
	var lies []string
	for k := range helpKeys() {
		if keyNav[k] || has(k) {
			continue
		}
		lies = append(lies, k)
	}
	sort.Strings(lies)
	if len(lies) > 0 {
		t.Errorf("a ajuda promete %d tecla(s) que ninguém trata: %s", len(lies), strings.Join(lies, " "))
	}
}

// TestEveryModuleHasAHelpBlock: módulo na sidebar é módulo com bloco na ajuda.
// Sem isto, um módulo novo nasce mudo — que foi o caso de JSON e JWT.
func TestEveryModuleHasAHelpBlock(t *testing.T) {
	blocks := map[string]bool{}
	for _, g := range helpGroups() {
		for _, b := range g.blocks {
			blocks[strings.ToUpper(b.title)] = true
		}
	}
	hasBlockFor := func(names ...string) bool {
		for _, n := range names {
			for title := range blocks {
				if strings.Contains(title, strings.ToUpper(n)) {
					return true
				}
			}
		}
		return false
	}
	// O nome curto da sidebar e o título do bloco nem sempre batem; o que não
	// pode é o módulo não ter bloco nenhum.
	alias := map[Tab]string{
		TabActions:   "GITHUB ACTIONS",
		TabCFTunnel:  "CLOUDFLARE TUNNEL",
		TabWebSocket: "WEBSOCKET",
	}
	for _, tab := range append(AllTabs, TabJSON, TabJWT) {
		if tab == TabOverview {
			continue // a Visão Geral é a landing do projeto, sem teclas próprias
		}
		name := tab.String()
		if a, ok := alias[tab]; ok {
			name = a
		}
		if !hasBlockFor(name) {
			t.Errorf("módulo %q não tem bloco na ajuda — nasce sem forma de descoberta", tab)
		}
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// TestCommandBarSpeaksOneLanguage: uma ação, um nome. "refresh", "atualizar",
// "recarregar" e "reescanear" eram quatro palavras para recarregar, em módulos
// diferentes — atalho memorizável começa por a ação ter um nome só, e em
// português (docs/DESIGN.md §10).
func TestCommandBarSpeaksOneLanguage(t *testing.T) {
	english := map[string]string{
		"refresh": "atualizar", "rescan": "reescanear", "restart": "reiniciar",
		"start": "iniciar", "stop": "parar", "delete": "excluir", "new": "novo",
		"edit": "editar", "copy": "copiar", "open": "abrir", "send": "enviar",
		"login": "entrar", "preview": "prévia", "trigger": "disparar",
		"decode": "decodificar", "verify": "verificar", "sign": "assinar",
		"export": "exportar",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	pairRe := regexp.MustCompile(`\[2\]string\{"([^"]+)", "([^"]+)"\}`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range pairRe.FindAllStringSubmatch(string(src), -1) {
			first := strings.ToLower(strings.Fields(m[2])[0])
			if pt, bad := english[first]; bad {
				t.Errorf("%s: a barra diz %q — em português é %q (§10)", f, m[2], pt)
			}
		}
	}
}

// TestNavigationVocabularyIsUniform: a mesma tecla de navegação recebe o mesmo
// nome em toda tela. `↑↓` já foi "navegar", "rolar", "lista" e "porta" na mesma
// versão — o que muda é o alvo, não a tecla.
func TestNavigationVocabularyIsUniform(t *testing.T) {
	// enter e esc mudam de alvo por contexto e por isso mudam de rótulo. As de
	// MOVIMENTO, não: elas sempre movem.
	uniform := map[string]bool{"pgup": true, "pgdown": true, "pgup/pgdown": true}
	files, _ := filepath.Glob("*.go")
	pairRe := regexp.MustCompile(`\[2\]string\{"([^"]+)", "([^"]+)"\}`)
	seen := map[string]map[string][]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, _ := os.ReadFile(f)
		for _, m := range pairRe.FindAllStringSubmatch(string(src), -1) {
			k := strings.TrimSpace(m[1])
			if !uniform[k] {
				continue
			}
			verb := strings.ToLower(strings.Fields(m[2])[0])
			if seen[k] == nil {
				seen[k] = map[string][]string{}
			}
			seen[k][verb] = append(seen[k][verb], f)
		}
	}
	for k, verbs := range seen {
		if len(verbs) > 1 {
			var names []string
			for v := range verbs {
				names = append(names, v)
			}
			sort.Strings(names)
			t.Errorf("a tecla %q é chamada de %s em telas diferentes — movimento tem um nome só",
				k, strings.Join(names, ", "))
		}
	}
}

// TestKeysAreSpelledTheSameEverywhere: a barra escrevia "S-R" e "^g" enquanto a
// ajuda escreve "shift+R" e "ctrl+g". Quem lê a ajuda e volta para a barra não
// reconhece o atalho — é a mesma doença dos verbos (sessão 8), na grafia.
func TestKeysAreSpelledTheSameEverywhere(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	// "^x" e "S-X" são as formas curtas; a forma do app é a que a ajuda usa.
	short := regexp.MustCompile(`"(\^[a-zA-Z]|S-[A-Za-z])"`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if m := short.FindString(line); m != "" {
				t.Errorf("%s:%d escreve a tecla como %s — o app escreve ctrl+x e shift+X",
					f, i+1, m)
			}
		}
	}
}

// TestStateHasOneWord: "Running" na sidebar e "rodando" no dashboard eram dois
// nomes para o mesmo estado do mesmo projeto — e um deles em inglês (§10).
func TestStateHasOneWord(t *testing.T) {
	for _, s := range []core.ProjectStatus{
		core.StatusRunning, core.StatusStopped, core.StatusDegraded, core.StatusUnknown,
	} {
		word := projectStatusWord(s)
		label := stripANSI(statusLabel(s, 0))
		if !strings.HasSuffix(label, word) {
			t.Errorf("estado %v: a sidebar diz %q e o dashboard diz %q", s, label, word)
		}
		if word != strings.ToLower(word) {
			t.Errorf("estado %v: %q — microcópia em minúsculas (§10)", s, word)
		}
	}
}
