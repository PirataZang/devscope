package hub

import (
	"strings"

	"rsc.io/qr"
)

// qrMargin é a borda clara, em módulos. Se o celular não ler o código, é o
// primeiro ajuste a fazer.
const qrMargin = 2

// QR desenha o texto como QR (correção M) com meio-blocos: cada linha de texto
// cobre duas linhas de módulos. Os módulos claros é que viram bloco, para o
// código sair escuro sobre claro num terminal de fundo escuro.
func QR(text string) ([]string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil, err
	}
	n := code.Size + 2*qrMargin
	light := func(x, y int) bool {
		x, y = x-qrMargin, y-qrMargin
		if x < 0 || y < 0 || x >= code.Size || y >= code.Size {
			return true
		}
		return !code.Black(x, y)
	}
	var lines []string
	for y := 0; y < n; y += 2 {
		var b strings.Builder
		for x := range n {
			top, bottom := light(x, y), y+1 < n && light(x, y+1)
			switch {
			case top && bottom:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bottom:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		lines = append(lines, b.String())
	}
	return lines, nil
}
