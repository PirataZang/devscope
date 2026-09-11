package ui

import (
	"strconv"

	"github.com/devscope/devscope/internal/core"
)

// A gaveta do Git.
//
// A tela principal mostrava CINCO caixas ao mesmo tempo — BRANCHES, COMMITS,
// ALTERAÇÕES, STASHES e LOG DE COMANDOS — para responder a três perguntas.
// Duas dessas caixas não são ambientes:
//
//   - LOG DE COMANDOS é um EVENTO. A saída do `git pull` interessa nos dez
//     segundos depois do pull, e depois vira ruído permanente no rodapé.
//   - STASHES é uma CONSULTA. Só existia acima de 96 colunas, e a barra de
//     comandos anunciava uma tecla `s` que não tinha handler nenhum.
//
// Agora as duas dividem uma faixa no pé da tela que só existe quando alguém
// pede — ou, no caso do log, quando o próprio git acabou de falar.
//
// A tela padrão fica com BRANCHES + COMMITS + ALTERAÇÕES: a prioridade da
// sessão, e duas caixas em vez de cinco.

type gitDrawer int

const (
	gitDrawerNone gitDrawer = iota
	gitDrawerLog
	gitDrawerStash
)

// gitDrawerHeight é a fatia que a gaveta toma do corpo. Fixa e modesta: ela
// empresta espaço de branches/commits, então não pode virar a tela.
func (a *App) gitDrawerHeight(bodyH int) int {
	if a.gitDrawer == gitDrawerNone {
		return 0
	}
	h := bodyH * 30 / 100
	if h < 5 {
		h = 5
	}
	if h > 12 {
		h = 12
	}
	// Nunca a ponto de sufocar branches/commits + alterações.
	if max := bodyH - 12; h > max {
		h = maxInt(0, max)
	}
	if h < 4 {
		return 0
	}
	return h
}

// openGitDrawer abre (ou troca) a gaveta. Abrir o log leva o foco junto: é
// para lá que o olho vai, e é onde `o` abre a URL do output.
func (a *App) openGitDrawer(d gitDrawer) {
	a.gitDrawer = d
	switch d {
	case gitDrawerLog:
		a.gitFocus = gitFocusCmdLog
	case gitDrawerStash:
		a.gitStashCursor = clampCursor(a.gitStashCursor, a.gitStashCount())
		if a.gitFocus == gitFocusCmdLog {
			a.gitFocus = gitFocusFiles
		}
	}
}

func (a *App) closeGitDrawer() {
	a.gitDrawer = gitDrawerNone
	if a.gitFocus == gitFocusCmdLog {
		a.gitFocus = gitFocusFiles
	}
}

// toggleGitDrawer é o que as teclas chamam: a mesma tecla abre e fecha.
func (a *App) toggleGitDrawer(d gitDrawer) {
	if a.gitDrawer == d {
		a.closeGitDrawer()
		return
	}
	a.openGitDrawer(d)
}

// noteGitCommandRan abre o log sozinho quando um comando termina. É o ponto
// inteiro da mudança: a saída aparece na hora em que interessa, em vez de
// ocupar o rodapé para sempre esperando que interesse.
func (a *App) noteGitCommandRan() {
	if a.tab != TabGit || a.gitSubview != gitSubviewMain {
		return
	}
	a.gitDrawer = gitDrawerLog
	a.gitCmdLogScroll = 0
	a.gitCmdLogCursor = 0
}

func (a *App) gitStashCount() int {
	if g := a.projectGitInfo(a.currentProject()); g != nil {
		return len(g.Stashes)
	}
	return 0
}

// gitDrawerHint é o que a barra de comandos diz sobre a gaveta: sempre a mesma
// tecla, e o rótulo conta o que há dentro.
func (a *App) gitDrawerHints(g *core.GitInfo) [][2]string {
	log := [2]string{"ctrl+l", "log de comandos"}
	if a.gitDrawer == gitDrawerLog {
		log[1] = "fechar log"
	}
	out := [][2]string{log}
	if g != nil && g.StashCount > 0 {
		stash := [2]string{"s", "stash " + strconv.Itoa(g.StashCount)}
		if a.gitDrawer == gitDrawerStash {
			stash[1] = "fechar stash"
		}
		out = append(out, stash)
	}
	return out
}
