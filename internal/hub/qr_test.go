package hub

import (
	"testing"

	"rsc.io/qr"
)

// grid desfaz os meio-blocos: true é módulo claro.
func grid(t *testing.T, lines []string, n int) [][]bool {
	g := make([][]bool, n)
	for y := range g {
		g[y] = make([]bool, n)
	}
	for row, l := range lines {
		r := []rune(l)
		if len(r) != n {
			t.Fatalf("linha %d com %d colunas, queria %d", row, len(r), n)
		}
		for x, c := range r {
			top := c == '█' || c == '▀'
			bottom := c == '█' || c == '▄'
			g[2*row][x] = top
			if 2*row+1 < n {
				g[2*row+1][x] = bottom
			}
		}
	}
	return g
}

func TestQR(t *testing.T) {
	link := "https://t.me/devscope_vps_bot?start=K7F-29Q"
	lines, err := QR(link)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := qr.Encode(link, qr.M)
	n := code.Size + 2*qrMargin
	if len(lines) != (n+1)/2 {
		t.Fatalf("%d linhas para %d módulos", len(lines), n)
	}
	g := grid(t, lines, n)

	// Margem clara em volta.
	for i := range n {
		for m := range qrMargin {
			if !g[m][i] || !g[n-1-m][i] || !g[i][m] || !g[i][n-1-m] {
				t.Fatalf("margem escura em %d", i)
			}
		}
	}
	// Os três padrões de canto: anel escuro, anel claro, miolo 3x3 escuro.
	finder := func(ox, oy int) {
		for y := range 7 {
			for x := range 7 {
				ring := min(x, y, 6-x, 6-y)
				wantDark := ring != 1
				if dark := !g[oy+y][ox+x]; dark != wantDark {
					t.Fatalf("padrão de canto em (%d,%d) errado no módulo (%d,%d)", ox, oy, x, y)
				}
			}
		}
	}
	m, far := qrMargin, qrMargin+code.Size-7
	finder(m, m)
	finder(far, m)
	finder(m, far)

	// E cada módulo do código sai como o encoder mandou.
	for y := range code.Size {
		for x := range code.Size {
			if g[m+y][m+x] == code.Black(x, y) {
				t.Fatalf("módulo (%d,%d) invertido", x, y)
			}
		}
	}
}
