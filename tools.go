package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validarCodigoAtivo: liga a validação de sintaxe/compilação em criar/editar_arquivo
// (garante código executável). Desligável com --validar-codigo=false.
var validarCodigoAtivo = true

// caminho seguro: impede escapar do diretório de trabalho.
func safePath(root, p string) (string, error) {
	full := filepath.Join(root, p)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("caminho fora do diretório de trabalho: %s", p)
	}
	return full, nil
}

// schema de uma ferramenta no formato function-calling.
func fn(name, desc string, props map[string]any, req []string) any {
	return map[string]any{"type": "function", "function": map[string]any{
		"name": name, "description": desc,
		"parameters": map[string]any{"type": "object", "properties": props, "required": req}}}
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

// toolDefs — o catálogo enviado ao modelo.
func toolDefs() []any {
	return []any{
		fn("ler_arquivo", "Lê o conteúdo de um arquivo do projeto.",
			map[string]any{"path": str("caminho relativo do arquivo")}, []string{"path"}),
		fn("listar", "Lista arquivos/pastas de um diretório do projeto.",
			map[string]any{"path": str("caminho relativo (padrão: raiz)")}, nil),
		fn("criar_arquivo", "Cria ou sobrescreve um arquivo com o conteúdo dado.",
			map[string]any{"path": str("caminho relativo"), "conteudo": str("conteúdo completo")},
			[]string{"path", "conteudo"}),
		fn("editar_arquivo", "Substitui um trecho exato por outro em um arquivo existente.",
			map[string]any{"path": str("caminho relativo"), "buscar": str("trecho exato a substituir"),
				"substituir": str("novo trecho")}, []string{"path", "buscar", "substituir"}),
		fn("parse_drawio", "Decodifica um diagrama draw.io (.drawio/.xml) e retorna nós e conexões.",
			map[string]any{"path": str("caminho do arquivo draw.io")}, []string{"path"}),
		fn("gerar_diagrama", "Salva um diagrama em Mermaid num arquivo (.md).",
			map[string]any{"path": str("caminho de saída, ex.: arquitetura.md"),
				"mermaid": str("código Mermaid (sem cercas ```)")}, []string{"path", "mermaid"}),
		fn("memorizar", "Grava um fato durável na memória do projeto (persiste entre sessões).",
			map[string]any{"titulo": str("título curto do fato"),
				"fato": str("o fato a lembrar (conciso)")}, []string{"titulo", "fato"}),
		fn("rodar_subagentes", "Executa várias sub-tarefas INDEPENDENTES em paralelo (subagentes) e "+
			"agrega os resultados. Use para decompor análises/pesquisas amplas em partes.",
			map[string]any{"tarefas": map[string]any{"type": "array",
				"items": map[string]any{"type": "string"},
				"description": "lista de sub-tarefas independentes e autocontidas"}},
			[]string{"tarefas"}),
	}
}

const maxLeitura = 12000 // chars por leitura, para não estourar o contexto

// executa uma ferramenta e devolve o resultado como texto.
// args = JSON dos argumentos (string, formato OpenAI).
func execTool(root, nome, args string) string {
	get := func(k string) string {
		var m map[string]any
		json.Unmarshal([]byte(args), &m)
		if v, ok := m[k].(string); ok {
			return v
		}
		return ""
	}
	switch nome {
	case "ler_arquivo":
		p, err := safePath(root, get("path"))
		if err != nil {
			return "ERRO: " + err.Error()
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "ERRO: " + err.Error()
		}
		s := string(b)
		if len(s) > maxLeitura {
			s = s[:maxLeitura] + "\n...[truncado]"
		}
		return s
	case "listar":
		p, err := safePath(root, get("path"))
		if err != nil {
			return "ERRO: " + err.Error()
		}
		ents, err := os.ReadDir(p)
		if err != nil {
			return "ERRO: " + err.Error()
		}
		var b strings.Builder
		for _, e := range ents {
			marca := ""
			if e.IsDir() {
				marca = "/"
			}
			b.WriteString(e.Name() + marca + "\n")
		}
		return b.String()
	case "criar_arquivo":
		p, err := safePath(root, get("path"))
		if err != nil {
			return "ERRO: " + err.Error()
		}
		if validarCodigoAtivo {
			if err := validarCodigo(get("path"), get("conteudo")); err != nil {
				return "ERRO: código não passou na validação (corrija e refaça): " + err.Error()
			}
		}
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(get("conteudo")), 0o644); err != nil {
			return "ERRO: " + err.Error()
		}
		return "OK: arquivo criado (" + get("path") + ")"
	case "editar_arquivo":
		p, err := safePath(root, get("path"))
		if err != nil {
			return "ERRO: " + err.Error()
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "ERRO: " + err.Error()
		}
		buscar, subst := get("buscar"), get("substituir")
		if !strings.Contains(string(b), buscar) {
			return "ERRO: trecho 'buscar' não encontrado no arquivo"
		}
		novo := strings.Replace(string(b), buscar, subst, 1)
		if validarCodigoAtivo {
			if err := validarCodigo(get("path"), novo); err != nil {
				return "ERRO: a edição deixaria o arquivo inválido (corrija): " + err.Error()
			}
		}
		if err := os.WriteFile(p, []byte(novo), 0o644); err != nil {
			return "ERRO: " + err.Error()
		}
		return "OK: arquivo editado (" + get("path") + ")"
	case "parse_drawio":
		p, err := safePath(root, get("path"))
		if err != nil {
			return "ERRO: " + err.Error()
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "ERRO: " + err.Error()
		}
		res, err := parseDrawio(string(b))
		if err != nil {
			return "ERRO: " + err.Error()
		}
		return res
	case "gerar_diagrama":
		p, err := safePath(root, get("path"))
		if err != nil {
			return "ERRO: " + err.Error()
		}
		merm := strings.TrimSpace(get("mermaid"))
		if err := validarMermaid(merm); err != nil {
			return "ERRO: Mermaid inválido (corrija e refaça, sem estruturas não suportadas): " + err.Error()
		}
		conteudo := "```mermaid\n" + merm + "\n```\n"
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
			return "ERRO: " + err.Error()
		}
		return "OK: diagrama salvo (" + get("path") + ")"
	case "memorizar":
		return memorizarFato(root, get("titulo"), get("fato"))
	}
	return "ERRO: ferramenta desconhecida: " + nome
}

// resumo curto da chamada para exibir no terminal.
func resumoChamada(nome, args string) string {
	var m map[string]any
	json.Unmarshal([]byte(args), &m)
	if p, ok := m["path"].(string); ok {
		return nome + "(" + p + ")"
	}
	return nome + "()"
}
