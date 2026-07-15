package main

// Agentes em PARALELO (client-side). O modelo pode chamar rodar_subagentes com uma
// lista de sub-tarefas independentes; cada subagente roda seu próprio loop (goroutine),
// com histórico próprio, e os resultados são agregados.
//
// RESSALVA HONESTA: localmente há UMA instância do modelo (CPU/uma GPU). Ollama/
// llama-server serializam a inferência, então o paralelo NÃO multiplica a vazão de
// tokens — o ganho é de DECOMPOSIÇÃO/organização e de sobrepor I/O + ferramentas.
// Um pool pequeno evita saturar a engine (que enfileira de qualquer forma).

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

const maxSubConc = 3 // subagentes simultâneos (a engine serializa; pool evita timeouts)

func parseTarefas(args string) []string {
	var m struct {
		Tarefas []string `json:"tarefas"`
	}
	json.Unmarshal([]byte(args), &m)
	return m.Tarefas
}

// semSubagentes remove a própria ferramenta de fan-out (evita aninhamento infinito).
func semSubagentes(tools []any) []any {
	out := make([]any, 0, len(tools))
	for _, t := range tools {
		if m, ok := t.(map[string]any); ok {
			if f, ok := m["function"].(map[string]any); ok && f["name"] == "rodar_subagentes" {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// rodarSubagentes executa as tarefas em paralelo (pool limitado) e agrega os resultados.
func rodarSubagentes(host, model, root string, tools []any, maxIter int, led *ledger,
	tarefas []string, imprimir bool) string {
	if len(tarefas) == 0 {
		return "ERRO: nenhuma sub-tarefa fornecida"
	}
	if imprimir {
		fmt.Printf("  ⇉ %d subagentes em paralelo (máx %d simultâneos; engine serializa)\n",
			len(tarefas), maxSubConc)
	}
	const instr = "Você é um SUBAGENTE do Vitrúvio. Foque APENAS na tarefa dada, seja " +
		"conciso e objetivo, e entregue um resultado final direto. Não faça perguntas."
	sub := semSubagentes(tools)
	sem := make(chan struct{}, maxSubConc)
	saidas := make([]string, len(tarefas))
	var wg sync.WaitGroup
	for i, tarefa := range tarefas {
		wg.Add(1)
		go func(i int, tarefa string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			hist := []Message{{Role: "system", Content: montarSistema(instr)}}
			_, saida := rodarAgente(host, model, root, sub, maxIter, led, "subagent", hist, tarefa, false)
			saidas[i] = saida
			if imprimir {
				fmt.Printf("  ✓ subagente %d/%d concluído\n", i+1, len(tarefas))
			}
		}(i, tarefa)
	}
	wg.Wait()
	var b strings.Builder
	for i, s := range saidas {
		b.WriteString(fmt.Sprintf("### Subagente %d — %s\n%s\n\n", i+1, tarefas[i], strings.TrimSpace(s)))
	}
	return strings.TrimSpace(b.String())
}
