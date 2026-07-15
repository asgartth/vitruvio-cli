package main

// Memória + contexto do Vitrúvio (client-side, no workspace):
//   - VITRUVIO.md            → contexto/convenções do projeto
//   - .vitruvio/memory/      → memória persistente (MEMORY.md índice + fatos)
//   - .vitruvio/sessions/    → histórico de sessão (retomar com --continue)
//   - compactação            → resume turnos antigos quando o contexto cresce
//
// SEGURANÇA: contexto e memória vêm do workspace (potencialmente influenciáveis),
// então entram como instruções de MENOR autoridade (junto do agente, SOB a
// constituição) — nunca como autoridade de sistema. Ver montarSistema.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxContexto = 8000        // limite do bloco de contexto+memória (janela pequena)
const limiteContextoChars = 48000 // heurística p/ disparar compactação (~janela 32k)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	s = slugRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	if s == "" {
		s = "fato"
	}
	return s
}

// contextoProjeto: VITRUVIO.md + memória persistente, como texto (menor autoridade).
func contextoProjeto(root string) string {
	var b strings.Builder
	if v, err := os.ReadFile(filepath.Join(root, "VITRUVIO.md")); err == nil {
		b.WriteString("### Contexto do projeto (VITRUVIO.md)\n")
		b.Write(v)
		b.WriteString("\n\n")
	}
	memDir := filepath.Join(root, ".vitruvio", "memory")
	var mem strings.Builder
	if idx, err := os.ReadFile(filepath.Join(memDir, "MEMORY.md")); err == nil {
		mem.WriteString(string(idx) + "\n")
	}
	arqs, _ := filepath.Glob(filepath.Join(memDir, "*.md"))
	for _, a := range arqs {
		if filepath.Base(a) == "MEMORY.md" {
			continue
		}
		if c, err := os.ReadFile(a); err == nil {
			mem.WriteString("\n" + string(c) + "\n")
		}
	}
	if mem.Len() > 0 {
		b.WriteString("### Memória do projeto (fatos persistentes)\n")
		b.WriteString(mem.String())
	}
	s := strings.TrimSpace(b.String())
	if len(s) > maxContexto {
		s = s[:maxContexto] + "\n...[memória truncada — considere consolidar]"
	}
	return s
}

// memorizarFato grava um fato na memória do projeto e atualiza o índice MEMORY.md.
func memorizarFato(root, titulo, fato string) string {
	memDir := filepath.Join(root, ".vitruvio", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return "ERRO: " + err.Error()
	}
	nome := slug(titulo) + ".md"
	conteudo := fmt.Sprintf("# %s\n\n%s\n", strings.TrimSpace(titulo), strings.TrimSpace(fato))
	if err := os.WriteFile(filepath.Join(memDir, nome), []byte(conteudo), 0o644); err != nil {
		return "ERRO: " + err.Error()
	}
	idxPath := filepath.Join(memDir, "MEMORY.md")
	idx, _ := os.ReadFile(idxPath)
	if !strings.Contains(string(idx), "]("+nome+")") {
		base := string(idx)
		if base == "" {
			base = "# Memória do projeto\n\n"
		}
		os.WriteFile(idxPath, []byte(base+fmt.Sprintf("- [%s](%s)\n", strings.TrimSpace(titulo), nome)), 0o644)
	}
	return "OK: memorizado (" + nome + ")"
}

// salvarSessao / carregarUltimaSessao: persistência do histórico p/ --continue.
func salvarSessao(root, id string, hist []Message) {
	dir := filepath.Join(root, ".vitruvio", "sessions")
	os.MkdirAll(dir, 0o755)
	if b, err := json.Marshal(hist); err == nil {
		os.WriteFile(filepath.Join(dir, id+".json"), b, 0o644)
	}
}

func carregarUltimaSessao(root string) ([]Message, string, bool) {
	arqs, _ := filepath.Glob(filepath.Join(root, ".vitruvio", "sessions", "*.json"))
	if len(arqs) == 0 {
		return nil, "", false
	}
	sort.Strings(arqs)
	ult := arqs[len(arqs)-1]
	b, err := os.ReadFile(ult)
	if err != nil {
		return nil, "", false
	}
	var hist []Message
	if json.Unmarshal(b, &hist) != nil || len(hist) == 0 {
		return nil, "", false
	}
	return hist, strings.TrimSuffix(filepath.Base(ult), ".json"), true
}

// compactar: se o contexto crescer demais, resume os turnos antigos num bloco,
// preservando o system (0) e o ÚLTIMO turno completo (não quebra pares tool/assistant).
func compactar(host, model string, hist []Message, led *ledger) []Message {
	tot := 0
	for _, m := range hist {
		tot += len(m.Content)
	}
	if tot < limiteContextoChars || len(hist) < 6 {
		return hist
	}
	lastUser := -1
	for i := len(hist) - 1; i >= 1; i-- {
		if hist[i].Role == "user" {
			lastUser = i
			break
		}
	}
	if lastUser <= 1 {
		return hist
	}
	var b strings.Builder
	for _, m := range hist[1:lastUser] {
		c := m.Content
		if len(c) > 1500 {
			c = c[:1500] + "…"
		}
		b.WriteString(m.Role + ": " + c + "\n")
	}
	resumoMsgs := []Message{
		{Role: "system", Content: "Resuma em português, de forma concisa, o histórico a seguir, preservando decisões tomadas, arquivos criados/editados e pendências. Não invente."},
		{Role: "user", Content: b.String()},
	}
	resp, u, err := chatTools(host, model, resumoMsgs, nil)
	led.registrar("compactacao", model, u)
	if err != nil {
		return hist
	}
	nova := []Message{hist[0], {Role: "system",
		Content: "## Resumo do contexto anterior (compactado)\n" + strings.TrimSpace(resp.Content)}}
	return append(nova, hist[lastUser:]...)
}
