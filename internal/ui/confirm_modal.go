package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// deleteConfirmOpts configures the shared y/n delete (or destructive) modal.
type deleteConfirmOpts struct {
	Brand    string
	Color    lipgloss.Color
	Title    string // e.g. "Excluir container"
	Subtitle string // e.g. "remove o container Docker"
	Label    string // field title: container, branch, processo…
	Target   string
	Detail   string
}

func renderDeleteConfirmBox(opts deleteConfirmOpts, width, height int) string {
	if opts.Title == "" {
		opts.Title = "Excluir"
	}
	if opts.Subtitle == "" {
		opts.Subtitle = "esta ação não pode ser desfeita facilmente"
	}
	if opts.Label == "" {
		opts.Label = "item"
	}
	if opts.Brand == "" {
		opts.Brand = "DEVSCOPE"
	}
	if opts.Color == "" {
		opts.Color = ColorWarning
	}

	boxW := minInt(width-4, maxInt(44, width*50/100))
	boxH := minInt(height-2, maxInt(12, 14))
	innerW := maxInt(28, boxW-6)
	lines := tunnelModalChrome(opts.Brand, opts.Color, opts.Title, opts.Subtitle, "", innerW)
	lines = append(lines, "")
	nameBox := renderApiTitledBox(opts.Label,
		[]string{StyleWarning.Bold(true).Render(truncate(firstNonEmpty(opts.Target, "—"), innerW-2))},
		innerW, 3, true,
	)
	lines = append(lines, strings.Split(nameBox, "\n")...)
	if opts.Detail != "" {
		lines = append(lines, "", StyleMuted.Render(truncate(opts.Detail, innerW)))
	}
	lines = append(lines, "",
		StyleMuted.Render("y confirma  ·  n/esc cancela"),
	)
	return tunnelModalBox(lines, boxW, boxH, opts.Color)
}

func renderTunnelDeleteConfirmBox(brand string, brandColor lipgloss.Color, target, detail string, width, height int) string {
	return renderDeleteConfirmBox(deleteConfirmOpts{
		Brand:    brand,
		Color:    brandColor,
		Title:    "Excluir túnel",
		Subtitle: "remove da config do projeto",
		Label:    "túnel",
		Target:   target,
		Detail:   detail,
	}, width, height)
}
