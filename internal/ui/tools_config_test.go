package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
)

// A barra de comandos precisa dizer o que a tecla realmente abre — o rótulo
// fixo "opencode" mentia para quem configurou outro agente.
func TestDashboardShowsConfiguredAITool(t *testing.T) {
	a := &App{
		width: 160, height: 40, view: ViewDashboard,
		cfg:      &config.Config{Tools: config.ToolsConfig{AI: "claude --continue"}},
		snapshot: core.Snapshot{Projects: []core.Project{{Name: "api", Path: "/var/www/api"}}},
	}
	if got := a.aiToolLabel(); got != "claude" {
		t.Fatalf("rótulo do atalho = %q, esperado claude", got)
	}
	view := stripANSI(a.renderDashboard())
	if !strings.Contains(view, "claude") {
		t.Fatalf("dashboard devia anunciar claude:\n%s", view)
	}
	if strings.Contains(view, "opencode") {
		t.Fatal("rótulo antigo continua na tela")
	}
}

// Sem tools.ai configurado e com mais de um agente instalado, Shift+O pergunta
// antes de abrir; a escolha é salva para não perguntar de novo.
func TestAIPickerSavesChoice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &App{width: 100, height: 30, view: ViewDashboard, cfg: &config.Config{}}
	a.aiPickerOn = true
	a.aiPickerOpts = []string{"claude", "opencode"}
	a.aiPickerIdx = 0
	a.aiPickerPath = "/var/www/api"

	a.updateAIPicker(tea.KeyMsg{Type: tea.KeyDown})
	a.updateAIPicker(tea.KeyMsg{Type: tea.KeyEnter})

	if a.aiPickerOn {
		t.Fatal("enter precisa fechar o seletor")
	}
	if a.cfg.Tools.AI != "opencode" {
		t.Fatalf("escolha não aplicada em memória: %q", a.cfg.Tools.AI)
	}
	saved, err := os.ReadFile(filepath.Join(home, ".config", "devscope", "user_config.txt"))
	if err != nil {
		t.Fatalf("user_config.txt não foi escrito: %v", err)
	}
	if !strings.Contains(string(saved), "AI: opencode") {
		t.Fatalf("a escolha não persistiu:\n%s", saved)
	}
}

// SaveValue faz merge: gravar tools.ai não pode apagar o tema já salvo.

// O arquivo criado pelo Shift+C tem que ensinar o que dá para configurar.

// Os atalhos saem na ordem das teclas (1..9 e 0 por último) e entrada inválida
// não vira linha fantasma na tela inicial.
func TestAppShortcutsOrderAndValidation(t *testing.T) {
	cfg := &config.Config{Apps: map[string]config.AppShortcut{
		"0": {Name: "Zero", Command: "zero"},
		"2": {Name: "Dois", Command: "dois"},
		"1": {Name: "Um", Command: "um &"},
		"9": {Command: "  htop  "},          // sem nome: usa o comando
		"7": {Name: "Sem comando"},          // ignorado
		"x": {Name: "Fora", Command: "nao"}, // tecla inexistente
	}}
	got := cfg.AppShortcuts()
	var keys, names []string
	for _, e := range got {
		keys = append(keys, e.Key)
		names = append(names, e.Name)
	}
	if strings.Join(keys, "") != "1290" {
		t.Fatalf("ordem das teclas = %v", keys)
	}
	if names[2] != "htop" {
		t.Fatalf("sem nome deve cair no comando: %v", names)
	}
	if _, ok := cfg.AppShortcutFor("7"); ok {
		t.Fatal("entrada sem comando não pode virar atalho")
	}
}

// A faixa de apps só existe quando há app cadastrado — sem isso seria uma linha
// vazia dizendo "configure atalhos".
func TestAppsStripOnlyWhenConfigured(t *testing.T) {
	a := &App{width: 140, height: 34, view: ViewDashboard, cfg: &config.Config{}}
	if got := a.renderAppsStrip(120); got != "" {
		t.Fatalf("sem apps a faixa não deve aparecer: %q", got)
	}
	a.cfg.Apps = map[string]config.AppShortcut{"1": {Name: "Obsidian", Command: "obsidian &"}}
	got := stripANSI(a.renderAppsStrip(120))
	if !strings.Contains(got, "1 Obsidian") {
		t.Fatalf("faixa devia mostrar a tecla e o nome: %q", got)
	}
}

// A tecla só dispara o que está cadastrado; dígito sem app não faz nada.
func TestDigitRunsConfiguredApp(t *testing.T) {
	a := &App{
		width: 140, height: 34, view: ViewDashboard,
		cfg: &config.Config{Apps: map[string]config.AppShortcut{
			"1": {Name: "Obsidian", Command: "obsidian &"},
		}},
	}
	if cmd := a.runAppShortcut("1", "/var/www/api"); cmd == nil {
		t.Fatal("tecla cadastrada precisa devolver um comando")
	}
	if cmd := a.runAppShortcut("5", "/var/www/api"); cmd != nil {
		t.Fatal("tecla sem app cadastrado não pode executar nada")
	}
}

// Shift+C abre o painel de campos: tema em lista, dez caixas de atalho e o
// campo da IA. Nada de decorar o formato do arquivo.
func TestUserConfigPanelEditsFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &App{width: 140, height: 40, view: ViewDashboard, cfg: &config.Config{}}
	a.openUserConfig()
	if !a.userCfgOn {
		t.Fatal("shift+C precisa abrir as preferências")
	}
	if a.userCfgField != prefFieldTheme {
		t.Fatal("o painel abre no tema")
	}

	// tema é lista: seta gira e aplica na hora
	before := a.userCfgPrefs.Theme
	a.updateUserConfig(tea.KeyMsg{Type: tea.KeyRight})
	if a.userCfgPrefs.Theme == before {
		t.Fatal("seta devia trocar o tema")
	}
	if CurrentTheme() != a.userCfgPrefs.Theme {
		t.Fatal("a troca de tema precisa aparecer na hora")
	}

	// tab leva para o nome do slot 1; digitar preenche
	a.updateUserConfig(tea.KeyMsg{Type: tea.KeyTab})
	if !prefIsName(a.userCfgField) || prefSlotOf(a.userCfgField) != 0 {
		t.Fatalf("depois do tema vem o nome do slot 1, veio campo %d", a.userCfgField)
	}
	// o slot 1 já vem com o exemplo de fábrica; o teste preenche o 2, vazio
	a.userCfgField, a.userCfgCursor = 1+1*prefFieldsPer, 0
	for _, r := range "Obsidian" {
		a.updateUserConfig(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	a.updateUserConfig(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "obsidian &" {
		a.updateUserConfig(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !a.userCfgDirty {
		t.Fatal("digitar precisa marcar como alterado")
	}

	// enter salva, aplica e escreve o arquivo com os comentários
	a.updateUserConfig(tea.KeyMsg{Type: tea.KeyEnter})
	if a.userCfgDirty {
		t.Fatal("enter devia salvar")
	}
	if app, ok := a.cfg.AppShortcutFor("2"); !ok || app.Name != "Obsidian" || app.Command != "obsidian &" {
		t.Fatalf("atalho não foi aplicado: %+v", a.cfg.Apps)
	}
	saved, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# THEME", "Obsidian", "obsidian &", "# AI"} {
		if !strings.Contains(string(saved), want) {
			t.Fatalf("arquivo sem %q:\n%s", want, saved)
		}
	}
}

// O último campo é a IA, e ela é texto — dá para escrever "claude --continue".
func TestUserConfigPanelAIField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &App{width: 140, height: 40, cfg: &config.Config{}}
	a.openUserConfig()
	a.userCfgField = prefFieldAI()
	a.userCfgPrefs.AI = ""
	a.userCfgCursor = 0
	for _, r := range "claude --continue" {
		a.updateUserConfig(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	a.updateUserConfig(tea.KeyMsg{Type: tea.KeyEnter})
	if a.cfg.Tools.AI != "claude --continue" {
		t.Fatalf("IA = %q", a.cfg.Tools.AI)
	}
}

// A cópia de fábrica fica ao lado, para dar para restaurar sem abrir o DevScope.
func TestFactoryCopyIsWrittenBesideTheConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &App{width: 120, height: 34, cfg: &config.Config{}}
	a.openUserConfig()

	b, err := os.ReadFile(config.DefaultUserConfigPath())
	if err != nil {
		t.Fatalf("cópia de fábrica não foi criada: %v", err)
	}
	prefs, warnings := config.ParseUserConfig(string(b))
	if len(warnings) != 0 || prefs.Theme != "dark" || len(prefs.Commands) != len(config.AppSlots) {
		t.Fatalf("a cópia precisa ser um arquivo válido e completo: %+v %v", prefs, warnings)
	}
	if config.DefaultUserConfigPath() == config.UserConfigPath() {
		t.Fatal("a cópia não pode sobrescrever o arquivo em uso")
	}
}

// Ctrl+O é a tecla anunciada na tela principal — ela tem que chegar ao handler
// do dashboard e abrir o agente de IA, não só o Shift+O.
func TestCtrlOOpensAIFromDashboard(t *testing.T) {
	p := core.Project{Name: "api", Path: "/var/www/api"}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyCtrlO},
		{Type: tea.KeyRunes, Runes: []rune("O")},
	} {
		a := &App{
			width: 160, height: 40, view: ViewDashboard,
			cfg:      &config.Config{Tools: config.ToolsConfig{AI: "sh -c true"}},
			snapshot: core.Snapshot{Projects: []core.Project{p}},
		}
		if _, cmd := a.updateDashboard(key); cmd == nil {
			t.Fatalf("%s não abriu o agente de IA", key.String())
		}
	}
}

// O rótulo da tela precisa dizer a tecla que existe.
func TestDashboardAnnouncesCtrlO(t *testing.T) {
	a := &App{
		width: 200, height: 40, view: ViewDashboard,
		cfg:      &config.Config{Tools: config.ToolsConfig{AI: "claude"}},
		snapshot: core.Snapshot{Projects: []core.Project{{Name: "api", Path: "/var/www/api"}}},
	}
	got := stripANSI(a.renderDashboard())
	if !strings.Contains(got, "ctrl+o claude") {
		t.Fatalf("a barra devia anunciar ctrl+o claude:\n%s", got)
	}
}

// Dentro do projeto o ctrl+O também abre a IA, na pasta do projeto aberto.
func TestCtrlOOpensAIInsideProject(t *testing.T) {
	p := core.Project{Name: "api", Path: "/var/www/api"}
	a := &App{
		width: 160, height: 40, view: ViewProject, tab: TabOverview,
		selectedProject: &p,
		cfg:             &config.Config{Tools: config.ToolsConfig{AI: "sh -c true"}},
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	if _, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyCtrlO}); cmd == nil {
		t.Fatal("ctrl+O dentro do projeto devia abrir o agente de IA")
	}
}

// A rolagem tem que acompanhar até o último slot e a IA: era aí que a caixa
// ficava cortada, porque a linha do campo em foco era estimada em vez de medida.
func TestUserConfigPanelScrollsToEveryField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for _, size := range []struct{ w, h int }{{88, 30}, {88, 42}, {140, 28}, {140, 44}} {
		a := &App{width: size.w, height: size.h, cfg: &config.Config{}}
		a.openUserConfig()

		for i := 0; i < prefFieldCount(); i++ {
			view := stripANSI(a.renderUserConfigScreen(""))
			if a.userCfgField == prefFieldTheme {
				if !strings.Contains(view, "←→ troca") {
					t.Fatalf("%dx%d: o tema em foco saiu da tela", size.w, size.h)
				}
			} else if !strings.Contains(view, "█") {
				t.Fatalf("%dx%d: campo %d em foco não está visível:\n%s",
					size.w, size.h, a.userCfgField, view)
			}
			a.moveUserCfgField(1)
		}
	}
}
