package main

// TUI de tela cheia (Bubble Tea): cabeçalho FIXO que se redimensiona ao vivo
// (WindowSizeMsg), transcrição rolável (viewport) e input (textinput) com palette
// de comandos "/". O loop do agente roda numa goroutine e envia eventos; a UI
// re-renderiza tudo a cada mensagem, então redimensionar NÃO quebra a interface.

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// cfgTUI: tudo que o loop do agente precisa (imutável durante a sessão).
type cfgTUI struct {
	host, model, root string
	tools             []any
	maxIter           int
	led               *ledger
	registry          map[string]Skill
	skillName         string
	instr             string // instruções combinadas (skill + contexto do projeto)
}

// estilos (Color 209 = mesmo salmão do modo ANSI)
var (
	stAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("209"))
	stMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	stDim    = lipgloss.NewStyle().Faint(true)
	stBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("209")).Padding(0, 1)
)

// evento vindo da goroutine do agente
type evTurno struct {
	tipo  string // assistant | tool | usage | done | erro
	texto string
	usage Usage
	hist  []Message
}

type entradaT struct{ tipo, texto string }

type modelo struct {
	cfg        cfgTUI
	larg, alt  int
	vp         viewport.Model
	entrada    textarea.Model
	sp         spinner.Model
	hist       []Message
	transcript []entradaT
	ocupado    bool
	turno      Usage
	sess       Usage
	ch         chan evTurno
	cancelar   context.CancelFunc
	palSel     int
	pronto     bool
}

// alturaInput: nº de linhas que o texto ocupa após o wrap (1..6).
func alturaInput(valor string, larg int) int {
	if larg < 1 {
		larg = 1
	}
	n := 0
	for _, ln := range strings.Split(valor, "\n") {
		n += lipgloss.Width(ln)/larg + 1
	}
	if n < 1 {
		n = 1
	}
	if n > 6 {
		n = 6
	}
	return n
}

func novoModelo(cfg cfgTUI, hist []Message) modelo {
	ta := textarea.New()
	ta.Placeholder = "descreva o que construir ou analisar (/ para comandos)"
	// prompt SEM ANSI (o textarea conta a largura do prompt; códigos de cor
	// quebrariam o cálculo de wrap). A cor vai pelo style.
	ta.Prompt = "› "
	ta.FocusedStyle.Prompt = stAccent
	ta.BlurredStyle.Prompt = stAccent
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.Focus()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = stAccent
	return modelo{cfg: cfg, entrada: ta, sp: sp, hist: hist, ch: make(chan evTurno, 8)}
}

func (m modelo) Init() tea.Cmd { return textarea.Blink }

// ---- palette de comandos ----

func (m modelo) matches() []cmdInfo {
	v := m.entrada.Value()
	if !strings.HasPrefix(v, "/") {
		return nil
	}
	var out []cmdInfo
	for _, c := range comandosPalette {
		if strings.HasPrefix(c.nome, v) {
			out = append(out, c)
		}
	}
	return out
}

// ---- render ----

func (m modelo) header() string {
	larg := m.larg
	if larg < 24 {
		larg = 24
	}
	inner := larg - 4 // 2 bordas + 2 padding
	sub := "arquiteto de soluções · software · negócios · segurança"
	if inner < 56 {
		sub = "arquiteto + segurança · pt-br"
	}
	// centraliza CADA linha do mascote (linhas simétricas → mantém o Λ)
	mlin := make([]string, len(mascote))
	for i, l := range mascote {
		mlin[i] = lipgloss.PlaceHorizontal(inner, lipgloss.Center, stAccent.Render(l))
	}
	masc := strings.Join(mlin, "\n")
	rot := func(k string) string { return stMuted.Render(k) }
	dir := m.cfg.root
	if lipgloss.Width(dir) > inner-8 {
		dir = "…" + dir[len(dir)-(inner-9):]
	}
	info := lipgloss.JoinVertical(lipgloss.Left,
		stAccent.Bold(true).Render("△ Vitrúvio")+" "+stMuted.Render(versao),
		stMuted.Render(sub),
		"",
		rot("modelo  ")+m.cfg.model,
		rot("engine  ")+m.cfg.host,
		rot("dir     ")+dir,
		rot("tokens  ")+fmt.Sprintf("%d entrada / %d saída (sessão)", m.sess.Prompt, m.sess.Completion),
	)
	conteudo := lipgloss.JoinVertical(lipgloss.Left, masc, "", info)
	return stBox.Width(inner + 2).Render(conteudo)
}

func (m modelo) statusLine() string {
	esq := stDim.Render("? /ajuda · Enter envia · Ctrl+C sai")
	dir := stAccent.Render(fmt.Sprintf("△ %d tokens", m.sess.Total))
	espaco := m.larg - lipgloss.Width(esq) - lipgloss.Width(dir)
	if espaco < 1 {
		espaco = 1
	}
	return esq + strings.Repeat(" ", espaco) + dir
}

func (m modelo) inputArea() string {
	linhas := []string{m.entrada.View()}
	if ms := m.matches(); len(ms) > 0 {
		for i, c := range ms {
			l := fmt.Sprintf("%-11s %s", c.nome, c.desc)
			if i == m.palSel {
				linhas = append(linhas, stAccent.Render("› "+l))
			} else {
				linhas = append(linhas, stMuted.Render("  "+l))
			}
		}
	}
	return strings.Join(linhas, "\n")
}

func renderTranscript(ent []entradaT, larg int) string {
	wrap := lipgloss.NewStyle().Width(larg)
	var b strings.Builder
	for _, e := range ent {
		switch e.tipo {
		case "user":
			b.WriteString(stAccent.Render("› ") + e.texto + "\n\n")
		case "assistant":
			b.WriteString(wrap.Render(e.texto) + "\n\n")
		case "tool":
			b.WriteString(stDim.Render("  → "+e.texto) + "\n")
		case "sistema":
			b.WriteString(stMuted.Render(e.texto) + "\n\n")
		}
	}
	return b.String()
}

func (m *modelo) recalcular() {
	if !m.pronto {
		return
	}
	hH := lipgloss.Height(m.header())
	iH := lipgloss.Height(m.inputArea())
	vpH := m.alt - hH - iH - 2
	if vpH < 3 {
		vpH = 3
	}
	m.vp.Width = m.larg - 1 // 1 coluna reservada p/ a barra de rolagem
	m.vp.Height = vpH
	m.vp.SetContent(renderTranscript(m.transcript, m.vp.Width))
	m.vp.GotoBottom()
}

// scrollbar: barra de rolagem vertical (1 coluna) ao lado da transcrição.
func (m modelo) scrollbar() string {
	h := m.vp.Height
	total := m.vp.TotalLineCount()
	var b strings.Builder
	if total <= h { // não há o que rolar: track discreto
		for i := 0; i < h; i++ {
			b.WriteString(stDim.Render("│"))
			if i < h-1 {
				b.WriteString("\n")
			}
		}
		return b.String()
	}
	thumb := h * h / total
	if thumb < 1 {
		thumb = 1
	}
	pos := 0
	if maxOff := total - h; maxOff > 0 {
		pos = (h - thumb) * m.vp.YOffset / maxOff
	}
	for i := 0; i < h; i++ {
		if i >= pos && i < pos+thumb {
			b.WriteString(stAccent.Render("█"))
		} else {
			b.WriteString(stDim.Render("│"))
		}
		if i < h-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m modelo) View() string {
	if !m.pronto {
		return "iniciando…"
	}
	entrada := m.inputArea()
	if m.ocupado {
		entrada = m.sp.View() + " " + stMuted.Render("pensando…") +
			stDim.Render(fmt.Sprintf("  (turno %d tokens · Esc interrompe)", m.turno.Total))
	}
	corpo := lipgloss.JoinHorizontal(lipgloss.Top, m.vp.View(), m.scrollbar())
	return lipgloss.JoinVertical(lipgloss.Left,
		m.header(), corpo, entrada, m.statusLine())
}

// ---- update ----

func aguardar(ch chan evTurno) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m modelo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.larg, m.alt = msg.Width, msg.Height
		m.entrada.SetWidth(m.larg - 2)
		m.entrada.SetHeight(alturaInput(m.entrada.Value(), m.larg-5))
		if !m.pronto {
			m.vp = viewport.New(m.larg-1, 3)
			m.pronto = true
		}
		m.recalcular()
		return m, nil

	case spinner.TickMsg:
		if m.ocupado {
			var c tea.Cmd
			m.sp, c = m.sp.Update(msg)
			return m, c
		}
		return m, nil

	case evTurno:
		switch msg.tipo {
		case "usage":
			m.turno = m.turno.add(msg.usage)
			m.sess = m.sess.add(msg.usage)
		case "assistant":
			if strings.TrimSpace(msg.texto) != "" {
				m.transcript = append(m.transcript, entradaT{"assistant", strings.TrimSpace(msg.texto)})
			}
		case "tool":
			m.transcript = append(m.transcript, entradaT{"tool", msg.texto})
		case "erro":
			m.transcript = append(m.transcript, entradaT{"assistant", "erro: " + msg.texto})
		case "cancelado":
			m.transcript = append(m.transcript, entradaT{"sistema", "· interrompido"})
		case "done":
			m.hist = msg.hist
			m.ocupado = false
			m.cancelar = nil
			salvarSessao(m.cfg.root, m.cfg.led.session, m.hist)
			m.recalcular()
			return m, nil
		}
		m.recalcular()
		return m, aguardar(m.ch) // continua lendo eventos do turno

	case tea.MouseMsg: // roda do mouse rola a transcrição
		var c tea.Cmd
		m.vp, c = m.vp.Update(msg)
		return m, c

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.ocupado {
				if m.cancelar != nil {
					m.cancelar() // aborta a requisição em curso; o turno finaliza em "done"
				}
				return m, nil
			}
			m.entrada.SetValue("") // ocioso: ESC limpa o input/palette
			m.palSel = 0
			return m, nil
		case tea.KeyPgUp, tea.KeyPgDown: // rola a transcrição
			var c tea.Cmd
			m.vp, c = m.vp.Update(msg)
			return m, c
		}
		if m.ocupado {
			return m, nil
		}
		ms := m.matches()
		switch msg.Type {
		case tea.KeyUp:
			if len(ms) > 0 && m.palSel > 0 {
				m.palSel--
			}
			return m, nil
		case tea.KeyDown:
			if len(ms) > 0 && m.palSel < len(ms)-1 {
				m.palSel++
			}
			return m, nil
		case tea.KeyTab:
			if len(ms) > 0 {
				m.entrada.SetValue(ms[m.palSel].nome)
				m.palSel = 0
			}
			return m, nil
		case tea.KeyEnter:
			texto := strings.TrimSpace(m.entrada.Value())
			if len(ms) > 0 && strings.HasPrefix(texto, "/") {
				texto = ms[m.palSel].nome
			}
			m.entrada.SetValue("")
			m.entrada.SetHeight(1)
			m.palSel = 0
			if texto == "" {
				return m, nil
			}
			if strings.HasPrefix(texto, "/") {
				return m.rodarComando(texto)
			}
			return m.enviar(texto)
		}
		m.palSel = 0
		var c tea.Cmd
		m.entrada, c = m.entrada.Update(msg)
		m.entrada.SetHeight(alturaInput(m.entrada.Value(), m.larg-5))
		m.recalcular()
		return m, c
	}

	var c tea.Cmd
	m.vp, c = m.vp.Update(msg)
	return m, c
}

// enviar dispara um turno do agente numa goroutine (cancelável) e lê eventos.
func (m modelo) enviar(texto string) (tea.Model, tea.Cmd) {
	m.hist = compactar(m.cfg.host, m.cfg.model, m.hist, m.cfg.led)
	m.hist = append(m.hist, Message{Role: "user", Content: texto})
	m.transcript = append(m.transcript, entradaT{"user", texto})
	m.ocupado = true
	m.turno = Usage{}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelar = cancel
	histCopia := append([]Message(nil), m.hist...)
	go rodarTurnoTUI(ctx, m.cfg, histCopia, m.ch)
	m.recalcular()
	return m, tea.Batch(aguardar(m.ch), m.sp.Tick)
}

// rodarComando trata os comandos "/" (não vão ao agente).
func (m modelo) rodarComando(linha string) (tea.Model, tea.Cmd) {
	add := func(t string) { m.transcript = append(m.transcript, entradaT{"sistema", t}) }
	switch strings.Fields(linha)[0] {
	case "/sair", "/quit", "/exit":
		return m, tea.Quit
	case "/limpar", "/clear":
		m.transcript = nil
		m.hist = novoHistInstr(m.cfg.instr)
	case "/ajuda", "/help", "/?":
		var b strings.Builder
		for _, c := range comandosPalette {
			b.WriteString(fmt.Sprintf("%-11s %s\n", c.nome, c.desc))
		}
		add(strings.TrimRight(b.String(), "\n"))
	case "/usage", "/tokens":
		porSessao, total := consolidar(m.cfg.root)
		add(fmt.Sprintf("sessão atual: %d entrada / %d saída / %d total\nworkspace: %d entrada / %d saída / %d total (%d sessões)",
			m.sess.Prompt, m.sess.Completion, m.sess.Total,
			total.Prompt, total.Completion, total.Total, len(porSessao)))
	case "/memoria", "/memory":
		ctx := contextoProjeto(m.cfg.root)
		if ctx == "" {
			add("(sem VITRUVIO.md nem memória neste workspace)")
		} else {
			add(ctx)
		}
	case "/skills", "/agents":
		if len(m.cfg.registry) == 0 {
			add("(nenhuma skill/agente)")
		} else {
			var b strings.Builder
			for _, s := range m.cfg.registry {
				b.WriteString(fmt.Sprintf("%s — %s\n", s.Name, s.Description))
			}
			add(strings.TrimRight(b.String(), "\n"))
		}
	case "/compactar", "/compact":
		antes := len(m.hist)
		m.hist = compactar(m.cfg.host, m.cfg.model, m.hist, m.cfg.led)
		add(fmt.Sprintf("histórico: %d → %d mensagens", antes, len(m.hist)))
	case "/continuar", "/continue":
		if h, id, ok := carregarUltimaSessao(m.cfg.root); ok {
			m.hist = h
			add(fmt.Sprintf("retomada a sessão %s (%d mensagens)", id, len(h)))
		} else {
			add("(nenhuma sessão salva)")
		}
	default:
		add("comando desconhecido; /ajuda")
	}
	m.recalcular()
	return m, nil
}

// rodarTurnoTUI espelha o loop do agente, mas emite eventos em vez de imprimir.
// Cancelável por ctx (ESC): aborta a rodada em curso e finaliza limpo.
func rodarTurnoTUI(ctx context.Context, cfg cfgTUI, hist []Message, ch chan evTurno) {
	var turno Usage
	cancelado := false
	for i := 0; i < cfg.maxIter; i++ {
		if ctx.Err() != nil {
			cancelado = true
			break
		}
		resp, u, err := chatToolsCtx(ctx, cfg.host, cfg.model, hist, cfg.tools)
		turno = turno.add(u)
		ch <- evTurno{tipo: "usage", usage: u}
		if err != nil {
			if ctx.Err() != nil {
				cancelado = true
			} else {
				ch <- evTurno{tipo: "erro", texto: err.Error()}
			}
			break
		}
		hist = append(hist, resp)
		if len(resp.ToolCalls) == 0 {
			ch <- evTurno{tipo: "assistant", texto: resp.Content}
			break
		}
		for _, tc := range resp.ToolCalls {
			// Ao cancelar, ainda respondemos CADA tool_call (com "[interrompido]")
			// para o histórico ficar válido no próximo turno.
			if cancelado || ctx.Err() != nil {
				cancelado = true
				hist = append(hist, Message{Role: "tool", ToolCallID: tc.ID, Content: "[interrompido]"})
				continue
			}
			ch <- evTurno{tipo: "tool", texto: resumoChamada(tc.Function.Name, tc.Function.Arguments)}
			var res string
			if tc.Function.Name == "rodar_subagentes" {
				res = rodarSubagentes(cfg.host, cfg.model, cfg.root, cfg.tools, cfg.maxIter,
					cfg.led, parseTarefas(tc.Function.Arguments), false)
			} else {
				res = execTool(cfg.root, tc.Function.Name, tc.Function.Arguments)
			}
			hist = append(hist, Message{Role: "tool", ToolCallID: tc.ID, Content: res})
		}
		if cancelado {
			break
		}
	}
	cfg.led.registrar("turn", cfg.model, turno)
	if cancelado {
		ch <- evTurno{tipo: "cancelado"}
	}
	ch <- evTurno{tipo: "done", hist: hist}
}

// iniciarTUI sobe o programa Bubble Tea (alt-screen).
func iniciarTUI(cfg cfgTUI, hist []Message) error {
	p := tea.NewProgram(novoModelo(cfg, hist), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
