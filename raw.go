package main

// Leitura de linha em modo "raw" (tecla a tecla), com PALETTE de comandos ao vivo
// ao digitar "/", navegação por setas, Tab para completar e histórico (↑/↓).
// Se o stdin não for um terminal (ex.: entrada por pipe), o chamador usa o
// fallback por linha (bufio.Scanner).

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

type cmdInfo struct{ nome, desc string }

var comandosPalette = []cmdInfo{
	{"/ajuda", "lista os comandos"},
	{"/usage", "consumo de tokens"},
	{"/memoria", "mostra o contexto/memória"},
	{"/skills", "skills e agentes do workspace"},
	{"/compactar", "resume o histórico (libera contexto)"},
	{"/limpar", "novo contexto (mantém o banner)"},
	{"/continuar", "retoma a última sessão"},
	{"/sair", "encerra"},
}

func terminalInterativo() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// larguraTerminal: colunas do terminal (fallback 80).
func larguraTerminal() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 10 {
		return w
	}
	return 80
}

// lerLinha lê uma linha em raw-mode. Retorna (linha, ok); ok=false em Ctrl-C/EOF.
func lerLinha(prompt string, historico *[]string) (string, bool) {
	fd := int(os.Stdin.Fd())
	estado, err := term.MakeRaw(fd)
	if err != nil {
		return "", false
	}
	defer term.Restore(fd, estado)
	reader := bufio.NewReader(os.Stdin)

	buf := []rune{}
	sel := 0
	hist := *historico
	hidx := len(hist)

	matches := func() []cmdInfo {
		s := string(buf)
		if !strings.HasPrefix(s, "/") {
			return nil
		}
		var m []cmdInfo
		for _, c := range comandosPalette {
			if strings.HasPrefix(c.nome, s) {
				m = append(m, c)
			}
		}
		return m
	}

	redraw := func() {
		ms := matches()
		if sel >= len(ms) {
			sel = 0
		}
		fmt.Print("\r\033[J") // início da linha + limpa daqui pra baixo
		fmt.Print(prompt + string(buf))
		for i, c := range ms {
			linha := fmt.Sprintf("%-11s %s", c.nome, c.desc)
			if i == sel {
				fmt.Print("\r\n" + cAccent("› "+linha))
			} else {
				fmt.Print("\r\n" + cMuted("  "+linha))
			}
		}
		if len(ms) > 0 { // recoloca o cursor no fim do input
			fmt.Printf("\033[%dA\r\033[%dC", len(ms), visLen(prompt)+len(buf))
		}
	}

	redraw()
	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			fmt.Print("\r\n")
			return "", false
		}
		ms := matches()
		switch r {
		case 3: // Ctrl-C
			fmt.Print("\r\n")
			return "", false
		case 13, 10: // Enter
			fmt.Print("\r\033[J")
			linha := string(buf)
			if len(ms) > 0 && strings.HasPrefix(linha, "/") {
				linha = ms[sel].nome
			}
			fmt.Print(prompt + linha + "\r\n")
			if strings.TrimSpace(linha) != "" {
				*historico = append(*historico, linha)
			}
			return linha, true
		case 127, 8: // Backspace
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
			sel = 0
		case 9: // Tab: completa a seleção
			if len(ms) > 0 {
				buf = []rune(ms[sel].nome)
				sel = 0
			}
		case 21: // Ctrl-U: limpa a linha
			buf = nil
			sel = 0
		case 27: // ESC: sequência de seta (ESC [ A/B/C/D)
			b1, _, e1 := reader.ReadRune()
			if e1 != nil || b1 != '[' {
				break
			}
			b2, _, e2 := reader.ReadRune()
			if e2 != nil {
				break
			}
			switch b2 {
			case 'A': // ↑
				if len(ms) > 0 {
					if sel > 0 {
						sel--
					}
				} else if hidx > 0 {
					hidx--
					buf = []rune(hist[hidx])
				}
			case 'B': // ↓
				if len(ms) > 0 {
					if sel < len(ms)-1 {
						sel++
					}
				} else if hidx < len(hist) {
					hidx++
					if hidx == len(hist) {
						buf = nil
					} else {
						buf = []rune(hist[hidx])
					}
				}
			}
		default:
			if r >= 32 {
				buf = append(buf, r)
				sel = 0
			}
		}
		redraw()
	}
}
