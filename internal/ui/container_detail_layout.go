package ui

import "strings"

// Organização do conteúdo antes do realce. O docker entrega tabela com colunas
// de larguras diferentes por linha, env fora de ordem e config em blocos: sem
// alinhar, cada linha começa num lugar e a leitura vira varredura.

// layoutDetailLines devolve as linhas já organizadas para a aba aberta.
func layoutDetailLines(tab containerDetailTab, lines []string) []string {
	switch tab {
	case containerDetailTabEnv:
		return alignOnSeparator(lines, '=', 30)
	case containerDetailTabConfig:
		return alignConfigBlocks(lines)
	case containerDetailTabTop:
		return alignTableColumns(lines)
	}
	return lines
}

// alignOnSeparator alinha os valores numa coluna, com o separador colado à
// chave. O teto existe para uma variável gigante não empurrar as outras 40.
func alignOnSeparator(lines []string, sep byte, maxPad int) []string {
	width := 0
	for _, line := range lines {
		if i := strings.IndexByte(line, sep); i > 0 && i <= maxPad {
			width = maxInt(width, i)
		}
	}
	if width == 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		j := strings.IndexByte(line, sep)
		if j <= 0 || j > maxPad {
			out[i] = line
			continue
		}
		out[i] = padRight(line[:j+1], width+1) + line[j+1:]
	}
	return out
}

// alignConfigBlocks alinha "chave: valor" por nível de indentação — os itens de
// Labels/Mounts/Ports formam um bloco próprio e não devem seguir o topo.
func alignConfigBlocks(lines []string) []string {
	widths := map[int]int{}
	for _, line := range lines {
		indent, key, ok := configKey(line)
		if ok {
			widths[indent] = maxInt(widths[indent], len(key))
		}
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		indent, key, ok := configKey(line)
		if !ok || widths[indent] <= len(key) {
			out[i] = line
			continue
		}
		out[i] = line[:indent] + padRight(key, minInt(widths[indent], 28)) + line[indent+len(key):]
	}
	return out
}

// configKey isola "chave:" no começo da linha. Valor com ":" dentro (uma URL,
// uma porta) não confunde: só conta o primeiro.
func configKey(line string) (indent int, key string, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	indent = len(line) - len(trimmed)
	i := strings.IndexByte(trimmed, ':')
	if i <= 0 || i+1 >= len(trimmed) || trimmed[i+1] != ' ' {
		return 0, "", false
	}
	return indent, trimmed[:i+1], true
}

// alignTableColumns alinha a saída tabular do `docker top` pelas colunas do
// cabeçalho. A última coluna (o comando) fica livre: ela contém espaços.
func alignTableColumns(lines []string) []string {
	if len(lines) < 2 {
		return lines
	}
	cols := len(strings.Fields(lines[0]))
	if cols < 2 {
		return lines
	}
	rows := make([][]string, len(lines))
	widths := make([]int, cols)
	for i, line := range lines {
		f := splitFieldsN(line, cols)
		rows[i] = f
		for c := 0; c < len(f)-1 && c < cols; c++ {
			widths[c] = maxInt(widths[c], len(f[c]))
		}
	}
	out := make([]string, len(lines))
	for i, f := range rows {
		if len(f) < 2 {
			out[i] = lines[i]
			continue
		}
		var b strings.Builder
		for c, v := range f {
			if c > 0 {
				b.WriteString("  ")
			}
			if c == len(f)-1 {
				b.WriteString(v)
				continue
			}
			b.WriteString(padRight(v, widths[c]))
		}
		out[i] = b.String()
	}
	return out
}

// splitFieldsN quebra em no máximo n campos; o resto vai inteiro no último.
func splitFieldsN(line string, n int) []string {
	fields := strings.Fields(line)
	if len(fields) <= n {
		return fields
	}
	return append(fields[:n-1:n-1], strings.Join(fields[n-1:], " "))
}
