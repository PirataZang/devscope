package ui

import (
	"net/url"
	"strings"
)

// ── Tabela de chave/valor ─────────────────────────────────────────────────────
//
// Params e Headers são a MESMA coisa na tela: uma tabela de linhas ligáveis, com
// uma caixinha à esquerda. É assim no Postman e no Thunder Client, e é o que
// permite desligar um parâmetro para testar sem perdê-lo — antes, aqui, mexer na
// query era editar a mão a string dentro da URL, e desligar um parâmetro era
// apagá-lo e digitar de novo depois.
//
// O ARMAZENAMENTO continua sendo texto: a query dentro da própria URL e os
// headers no bloco `Chave: valor`. A tabela é uma VISTA, remontada da fonte e
// serializada de volta a cada edição. Duas consequências que valem o desenho:
//
//   - o cliente HTTP (api_client.go) não sabe que a tela mudou — `# ` na frente
//     de um header já era comentário ignorado por parseAPIHeaders, então linha
//     desligada não precisou de campo novo em lugar nenhum;
//   - URL colada de fora aparece como linhas na hora, e linha editada volta pra
//     URL na hora. Não existe "aplicar".

type apiKV struct {
	On    bool
	Key   string
	Value string
}

// apiSplitURL separa a URL em parte fixa e query, sem tentar validar nada: aqui
// se lida com URL em construção, e metade das vezes ela ainda está errada.
func apiSplitURL(raw string) (base, query string) {
	if i := strings.IndexByte(raw, '?'); i >= 0 {
		return raw[:i], raw[i+1:]
	}
	return raw, ""
}

// apiUnescapePct desfaz %XX e nada mais. url.QueryUnescape também troca `+` por
// espaço, e quem está depurando uma API com `+` no filtro perde o valor.
func apiUnescapePct(s string) string {
	if !strings.ContainsRune(s, '%') {
		return s
	}
	if out, err := url.PathUnescape(s); err == nil {
		return out
	}
	return s
}

// apiEscapeParam escapa só o que ATRAPALHA numa query. url.QueryEscape
// reescreveria `:` e `/` e a URL voltaria diferente da que o usuário colou —
// jeito rápido de perder a confiança de quem está depurando.
func apiEscapeParam(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '%':
			b.WriteString("%25")
		case ' ':
			b.WriteString("%20")
		case '&':
			b.WriteString("%26")
		case '=':
			b.WriteString("%3D")
		case '#':
			b.WriteString("%23")
		case '?':
			b.WriteString("%3F")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// apiParseQuery lê a query preservando a ORDEM. net/url devolve mapa, e mapa
// embaralha as linhas da tabela a cada tecla.
func apiParseQuery(q string) []apiKV {
	q = strings.TrimPrefix(strings.TrimSpace(q), "?")
	if q == "" {
		return nil
	}
	var rows []apiKV
	for _, part := range strings.Split(q, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		if strings.TrimSpace(k) == "" {
			continue
		}
		rows = append(rows, apiKV{On: true, Key: apiUnescapePct(k), Value: apiUnescapePct(v)})
	}
	return rows
}

func apiBuildQuery(rows []apiKV) string {
	var parts []string
	for _, r := range rows {
		if !r.On || strings.TrimSpace(r.Key) == "" {
			continue
		}
		p := apiEscapeParam(strings.TrimSpace(r.Key))
		if r.Value != "" {
			p += "=" + apiEscapeParam(r.Value)
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "&")
}

func apiParseHeaderRows(text string) []apiKV {
	var rows []apiKV
	for _, line := range strings.Split(text, "\n") {
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}
		on := true
		if strings.HasPrefix(raw, "#") {
			on, raw = false, strings.TrimSpace(strings.TrimPrefix(raw, "#"))
		}
		k, v, ok := strings.Cut(raw, ":")
		if !ok {
			k, v = raw, ""
		}
		if strings.TrimSpace(k) == "" {
			continue
		}
		rows = append(rows, apiKV{On: on, Key: strings.TrimSpace(k), Value: strings.TrimSpace(v)})
	}
	return rows
}

// apiBuildHeaderText serializa a tabela de volta pro bloco de texto. Linha
// desligada vira comentário — parseAPIHeaders já pula `#`.
func apiBuildHeaderText(rows []apiKV) string {
	var b strings.Builder
	for _, r := range rows {
		if strings.TrimSpace(r.Key) == "" {
			continue
		}
		if !r.On {
			b.WriteString("# ")
		}
		b.WriteString(strings.TrimSpace(r.Key) + ": " + r.Value + "\n")
	}
	return b.String()
}

// ── Ligação com o estado ──────────────────────────────────────────────────────

// apiSyncParamsFromURL relê a tabela a partir da URL. As linhas DESLIGADAS não
// estão na URL, então elas são preservadas na posição em que estavam: é o que
// permite desligar um parâmetro, mexer na URL à mão e ainda achar o parâmetro
// ali, esperando.
func (a *App) apiSyncParamsFromURL() {
	_, q := apiSplitURL(a.apiURL)
	fresh := apiParseQuery(q)
	var off []apiKV
	for _, r := range a.apiParams {
		if !r.On && strings.TrimSpace(r.Key) != "" {
			off = append(off, r)
		}
	}
	a.apiParams = append(fresh, off...)
	a.apiClampRows()
}

// apiSyncURLFromParams reescreve a query da URL a partir da tabela.
func (a *App) apiSyncURLFromParams() {
	base, _ := apiSplitURL(a.apiURL)
	if q := apiBuildQuery(a.apiParams); q != "" {
		a.apiURL = base + "?" + q
		return
	}
	a.apiURL = base
}

// apiRows devolve as linhas da aba ativa. Params moram no estado; headers são
// lidos do texto na hora, que é a fonte que o cliente HTTP usa.
func (a *App) apiRows() []apiKV {
	if a.apiSection == apiSecHeaders {
		return apiParseHeaderRows(a.apiHeaders)
	}
	return a.apiParams
}

// apiSetRows grava a tabela de volta na fonte da aba ativa.
func (a *App) apiSetRows(rows []apiKV) {
	if a.apiSection == apiSecHeaders {
		a.apiHeaders = apiBuildHeaderText(rows)
		a.apiClampRows()
		return
	}
	a.apiParams = rows
	a.apiSyncURLFromParams()
	a.apiClampRows()
}

func (a *App) apiClampRows() {
	n := len(a.apiRows())
	if a.apiRowCursor > n { // n = linha "nova", uma além do fim
		a.apiRowCursor = n
	}
	if a.apiRowCursor < 0 {
		a.apiRowCursor = 0
	}
	if a.apiRowField < 0 {
		a.apiRowField = 0
	}
	if a.apiRowField > 1 {
		a.apiRowField = 1
	}
}

// apiRowAdd abre uma linha vazia no fim e leva o cursor pra ela.
func (a *App) apiRowAdd() {
	rows := append(append([]apiKV{}, a.apiRows()...), apiKV{On: true})
	a.apiRowCursor = len(rows) - 1
	a.apiRowField = 0
	a.apiSetRows(rows)
}

func (a *App) apiRowDelete() {
	rows := a.apiRows()
	if a.apiRowCursor >= len(rows) {
		return
	}
	out := append(append([]apiKV{}, rows[:a.apiRowCursor]...), rows[a.apiRowCursor+1:]...)
	a.apiSetRows(out)
	if a.apiRowCursor > len(out) {
		a.apiRowCursor = len(out)
	}
}

// apiRowToggle liga/desliga a linha: no Postman é a caixinha da esquerda, e é o
// que faz a diferença entre testar sem um parâmetro e perdê-lo.
func (a *App) apiRowToggle() {
	rows := append([]apiKV{}, a.apiRows()...)
	if a.apiRowCursor >= len(rows) {
		return
	}
	rows[a.apiRowCursor].On = !rows[a.apiRowCursor].On
	a.apiSetRows(rows)
}

// apiRowText é o conteúdo da célula em foco, e apiRowSetText grava de volta.
// São eles que ligam a tabela ao editor de linha único já existente.
func (a *App) apiRowText() string {
	rows := a.apiRows()
	if a.apiRowCursor >= len(rows) {
		return ""
	}
	if a.apiRowField == 1 {
		return rows[a.apiRowCursor].Value
	}
	return rows[a.apiRowCursor].Key
}

func (a *App) apiRowSetText(text string) {
	rows := append([]apiKV{}, a.apiRows()...)
	for len(rows) <= a.apiRowCursor {
		rows = append(rows, apiKV{On: true})
	}
	if a.apiRowField == 1 {
		rows[a.apiRowCursor].Value = text
	} else {
		rows[a.apiRowCursor].Key = text
	}
	a.apiSetRows(rows)
}

// apiEnabledCount conta as linhas ligadas — vai no número da aba, como no
// Postman, pra você saber que tem header ativo sem abrir a aba.
func apiEnabledCount(rows []apiKV) int {
	n := 0
	for _, r := range rows {
		if r.On && strings.TrimSpace(r.Key) != "" {
			n++
		}
	}
	return n
}

// ── Copiar como curl ──────────────────────────────────────────────────────────

// apiCurlCommand monta o comando equivalente. É o atalho que mais se usa no
// Postman na vida real: você monta na tela e leva pro terminal, pro colega ou
// pro ticket.
func (a *App) apiCurlCommand() string {
	method := strings.ToUpper(strings.TrimSpace(a.apiMethod))
	if method == "" {
		method = "GET"
	}
	var b strings.Builder
	b.WriteString("curl -i -X " + method)
	b.WriteString(" '" + strings.ReplaceAll(strings.TrimSpace(a.apiURL), "'", "'\\''") + "'")
	for _, r := range apiParseHeaderRows(a.apiHeaders) {
		if !r.On {
			continue
		}
		b.WriteString(" \\\n  -H '" + r.Key + ": " + r.Value + "'")
	}
	switch a.apiAuthType {
	case apiAuthBearer:
		if tok := strings.TrimSpace(a.apiAuthToken); tok != "" {
			b.WriteString(" \\\n  -H 'Authorization: Bearer " + tok + "'")
		}
	case apiAuthBasic:
		if u := strings.TrimSpace(a.apiAuthUser); u != "" {
			b.WriteString(" \\\n  -u '" + u + ":" + a.apiAuthPass + "'")
		}
	}
	if body := strings.TrimSpace(a.apiBody); body != "" && methodAllowsBody(method) {
		b.WriteString(" \\\n  --data-raw '" + strings.ReplaceAll(body, "'", "'\\''") + "'")
	}
	return b.String()
}
