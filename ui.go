package main

// Experiência de terminal ao estilo Claude Code: banner de boas-vindas (box com
// marca, modelo, engine, dir e dicas), spinner de status com tempo + tokens, e
// comandos com "/". Cores ANSI (desligáveis com NO_COLOR ou --no-color).

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const versao = "v2.0"

var semCor = os.Getenv("NO_COLOR") != ""

const (
	aReset  = "\033[0m"
	aBold   = "\033[1m"
	aDim    = "\033[2m"
	aAccent = "\033[38;5;209m" // salmão/telha (nod ao Claude)
	aMuted  = "\033[38;5;245m"
	aOk     = "\033[38;5;114m"
)

func c(s, code string) string {
	if semCor {
		return s
	}
	return code + s + aReset
}
func cAccent(s string) string { return c(s, aAccent) }
func cMuted(s string) string  { return c(s, aMuted) }
func cBold(s string) string   { return c(s, aBold) }
func cDim(s string) string    { return c(s, aDim) }
func cOk(s string) string     { return c(s, aOk) }

var ansiRe = regexp.MustCompile("\033\\[[0-9;]*m")

// visLen: comprimento VISÍVEL (ignora códigos ANSI, conta runas) — p/ alinhar o box.
func visLen(s string) int { return utf8.RuneCountInString(ansiRe.ReplaceAllString(s, "")) }

// ajustarVis preenche (ou trunca) a string para EXATAMENTE n colunas visíveis,
// ignorando os códigos ANSI na contagem. Trunca só em janelas muito estreitas.
func ajustarVis(s string, n int) string {
	v := visLen(s)
	if v <= n {
		return s + strings.Repeat(" ", n-v)
	}
	plain := ansiRe.ReplaceAllString(s, "")
	r := []rune(plain)
	if n >= 1 {
		return string(r[:n-1]) + "…"
	}
	return ""
}

// caixaLarg desenha um box com largura INTERNA fixa `inner` (para preencher a
// largura total do terminal). Cada linha é ajustada a `inner` (sem quebrar).
func caixaLarg(linhas []string, inner int) string {
	if inner < 10 {
		inner = 10
	}
	bar := strings.Repeat("─", inner+2)
	var b strings.Builder
	b.WriteString(cAccent("╭"+bar+"╮") + "\n")
	for _, l := range linhas {
		b.WriteString(cAccent("│") + " " + ajustarVis(l, inner) + " " + cAccent("│") + "\n")
	}
	b.WriteString(cAccent("╰"+bar+"╯") + "\n")
	return b.String()
}

// mascote: o compasso do arquiteto (pixel art) — cabo no topo, dobradiça com furo,
// barra de ajuste e as pernas abrindo em Λ. Linhas SIMÉTRICAS (centralizadas uma a
// uma), então o formato se mantém tanto na TUI quanto no banner ANSI.
var mascote = []string{
	"███",
	"▟███▙",
	"██ ██",
	"██   ██",
	"██▄▄▄██",
	"██     ██",
	"██       ██",
	"▀▀       ▀▀",
}

func imprimirBanner(model, host, root string, nSkills int, skill string) {
	// responsivo à largura do terminal: textos mais curtos quando estreito.
	larg := larguraTerminal()
	dirMax := larg - 14
	if dirMax < 20 {
		dirMax = 20
	}
	if visLen(root) > dirMax {
		root = "…" + root[len(root)-(dirMax-1):]
	}
	sub := "arquiteto de soluções · software · negócios · segurança"
	tips := "/ajuda comandos · /usage tokens · /continuar retomar · /sair"
	if larg < 66 {
		sub = "arquiteto + segurança · pt-br"
		tips = "/ajuda · /usage · /sair"
	}
	rot := func(k string) string { return cMuted(k) }
	txt := []string{
		cBold(cAccent("△ Vitrúvio")) + " " + cMuted(versao),
		cMuted(sub),
		"",
		rot("modelo  ") + model,
		rot("engine  ") + host,
		rot("dir     ") + root,
		rot("skills  ") + fmt.Sprintf("%d", nSkills),
	}
	if skill != "" {
		txt = append(txt, rot("skill   ")+skill)
	}
	txt = append(txt, "", cDim(tips))

	// box em LARGURA TOTAL: inner = colunas do terminal - bordas/padding.
	inner := larg - 4
	if inner < 24 {
		inner = 24
	}
	linhas := make([]string, 0, len(mascote)+len(txt)+1)
	for _, l := range mascote { // mascote centralizado na largura total
		pad := (inner - visLen(l)) / 2
		if pad < 0 {
			pad = 0
		}
		linhas = append(linhas, strings.Repeat(" ", pad)+cAccent(l))
	}
	linhas = append(linhas, "")
	linhas = append(linhas, txt...)
	fmt.Print("\n" + caixaLarg(linhas, inner))
}

func textoAjuda() string {
	cmds := [][2]string{
		{"/ajuda", "lista os comandos"},
		{"/usage", "consumo de tokens (sessão + workspace)"},
		{"/memoria", "mostra o contexto/memória carregados"},
		{"/skills", "lista skills e agentes do workspace"},
		{"/compactar", "resume o histórico agora (libera contexto)"},
		{"/limpar", "limpa a conversa (novo contexto)"},
		{"/continuar", "retoma a última sessão salva"},
		{"/sair", "encerra"},
	}
	var b strings.Builder
	b.WriteString(cBold("Comandos") + "\n")
	for _, kv := range cmds {
		b.WriteString(fmt.Sprintf("  %s  %s\n", cAccent(fmt.Sprintf("%-11s", kv[0])), cMuted(kv[1])))
	}
	b.WriteString(cDim("O modelo também usa ferramentas: ler/criar/editar, memorizar, rodar_subagentes.") + "\n")
	return b.String()
}

// spinner: linha de status animada (frame · rótulo · tempo · tokens) enquanto a
// engine processa. tokens() dá o total acumulado do turno até agora.
type spinnerAnsi struct {
	stop chan struct{}
	done chan struct{}
}

func iniciarSpinner(rotulo string, tokens func() int) *spinnerAnsi {
	s := &spinnerAnsi{stop: make(chan struct{}), done: make(chan struct{})}
	if semCor {
		return s // sem animação em modo sem-cor; para() vira no-op via canais
	}
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		t0 := time.Now()
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Print("\r\033[K")
				close(s.done)
				return
			case <-time.After(100 * time.Millisecond):
				el := int(time.Since(t0).Seconds())
				fmt.Printf("\r%s %s %s", cAccent(frames[i%len(frames)]), rotulo,
					cDim(fmt.Sprintf("(%ds · ↑ %d tokens · Ctrl-C interrompe)", el, tokens())))
				i++
			}
		}
	}()
	return s
}

func (s *spinnerAnsi) parar() {
	if semCor {
		return
	}
	close(s.stop)
	<-s.done
}
