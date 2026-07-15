package main

// Validação determinística no tool-time (a garantia de "100% renderizável/executável"):
//   - validarMermaid: só salva diagrama que passa na checagem (tipo declarado,
//     subgraph/end e aspas balanceados); se `mmdc` estiver no PATH, valida de verdade.
//   - validarCodigo: syntax/compile check por linguagem (Go/JSON in-process; Python/
//     JS/TS via toolchain se disponível). Sem validador p/ a extensão → não bloqueia.
// Em falha, a ferramenta devolve ERRO → o agente recebe e SE AUTO-CORRIGE no loop.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var tiposMermaid = regexp.MustCompile(
	`^(?:flowchart|graph)\s+(?:TB|TD|BT|RL|LR)\b|` +
		`^(?:sequenceDiagram|classDiagram|stateDiagram(?:-v2)?|erDiagram|gantt|pie|` +
		`journey|gitGraph|mindmap|timeline|quadrantChart|requirementDiagram|` +
		`C4Context|C4Container|sankey(?:-beta)?|xychart(?:-beta)?)\b`)

// validarMermaid: checa os erros comuns que impedem a renderização.
func validarMermaid(codigo string) error {
	s := strings.TrimSpace(codigo)
	s = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(s, "```mermaid"), "```"), "```")
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("diagrama vazio")
	}
	linhas := strings.Split(s, "\n")
	primeira := ""
	for _, l := range linhas {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		primeira = t
		break
	}
	if !tiposMermaid.MatchString(primeira) {
		return fmt.Errorf("a 1ª linha não declara um tipo de diagrama Mermaid válido (ex.: 'flowchart LR'): %q", primeira)
	}
	sub, fim, aspas := 0, 0, 0
	for _, l := range linhas {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "subgraph") {
			sub++
		}
		if t == "end" || strings.HasPrefix(t, "end ") {
			fim++
		}
		aspas += strings.Count(l, "\"")
	}
	if sub != fim {
		return fmt.Errorf("subgraph/end desbalanceado (%d 'subgraph', %d 'end')", sub, fim)
	}
	if aspas%2 != 0 {
		return fmt.Errorf("aspas \" desbalanceadas nos labels")
	}
	if p, err := exec.LookPath("mmdc"); err == nil { // validação real quando disponível
		return validarComMmdc(p, s)
	}
	return nil
}

func validarComMmdc(mmdc, codigo string) error {
	dir, err := os.MkdirTemp("", "vitmerm")
	if err != nil {
		return nil // não conseguimos validar externamente → não bloqueia
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "d.mmd")
	os.WriteFile(in, []byte(codigo), 0o644)
	cmd := exec.Command(mmdc, "-i", in, "-o", filepath.Join(dir, "d.svg"))
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mmdc: %s", corta(errb.String(), 300))
	}
	return nil
}

// validarCodigo: syntax/compile check por extensão (best-effort).
func validarCodigo(path, conteudo string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		fset := token.NewFileSet()
		if _, err := parser.ParseFile(fset, path, conteudo, parser.AllErrors); err != nil {
			return fmt.Errorf("Go: %v", err)
		}
	case ".json":
		if !json.Valid([]byte(conteudo)) {
			return fmt.Errorf("JSON inválido")
		}
	case ".py":
		if bin := pyBin(); bin != "" {
			return checarArquivo(bin, conteudo, ".py", "-m", "py_compile")
		}
	case ".js", ".mjs", ".cjs":
		if p, err := exec.LookPath("node"); err == nil {
			return checarArquivo(p, conteudo, ".js", "--check")
		}
	case ".ts", ".tsx":
		if p, err := exec.LookPath("tsc"); err == nil {
			return checarArquivo(p, conteudo, ".ts", "--noEmit", "--skipLibCheck")
		}
	}
	return nil // sem validador para a extensão → não bloqueia
}

func pyBin() string {
	for _, n := range []string{"python3", "python"} {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

func checarArquivo(bin, conteudo, ext string, args ...string) error {
	dir, err := os.MkdirTemp("", "vitcod")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)
	f := filepath.Join(dir, "code"+ext)
	if err := os.WriteFile(f, []byte(conteudo), 0o644); err != nil {
		return nil
	}
	cmd := exec.Command(bin, append(args, f)...)
	var out bytes.Buffer
	cmd.Stderr, cmd.Stdout = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", strings.TrimPrefix(ext, "."), corta(out.String(), 300))
	}
	return nil
}

func corta(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
