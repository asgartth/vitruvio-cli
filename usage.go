package main

// Contabilidade de tokens (entrada/saída/cache) por atividade, consolidada por
// SESSÃO e por WORKSPACE. A engine (Ollama/llama-server) devolve `usage` em cada
// resposta /v1/chat/completions; aqui capturamos, acumulamos e persistimos num
// ledger append-only sob .vitruvio/usage/<sessao>.jsonl (no workspace).
//
// Nota honesta sobre "cache": localmente não há billing de cache como na API
// Anthropic. llama-server pode reaproveitar KV-cache do prompt; quando a engine
// reporta tokens de cache, registramos em Cached — senão fica 0.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Usage: tokens de uma atividade. Tags casam com o `usage` da OpenAI/Ollama.
type Usage struct {
	Prompt     int `json:"prompt_tokens"`
	Completion int `json:"completion_tokens"`
	Total      int `json:"total_tokens"`
	Cached     int `json:"cached_tokens,omitempty"`
}

func (u Usage) add(o Usage) Usage {
	return Usage{u.Prompt + o.Prompt, u.Completion + o.Completion,
		u.Total + o.Total, u.Cached + o.Cached}
}

// normaliza: se a engine não mandou Total, deriva de prompt+completion.
func (u Usage) norm() Usage {
	if u.Total == 0 {
		u.Total = u.Prompt + u.Completion
	}
	return u
}

type usageRecord struct {
	TS      string `json:"ts"`
	Session string `json:"session"`
	Kind    string `json:"kind"` // turn | subagent | serve
	Model   string `json:"model"`
	Usage
}

// ledger: acumulador da sessão + persistência append-only no workspace.
type ledger struct {
	mu      sync.Mutex
	dir     string
	session string
	sess    Usage
}

func novoLedger(root string) *ledger {
	dir := filepath.Join(root, ".vitruvio", "usage")
	os.MkdirAll(dir, 0o755)
	return &ledger{dir: dir, session: time.Now().Format("20060102-150405")}
}

// registrar acumula na sessão e grava uma linha no ledger. Concorrência-seguro
// (usado também pelos subagentes em paralelo).
func (l *ledger) registrar(kind, model string, u Usage) Usage {
	if l == nil {
		return u.norm()
	}
	u = u.norm()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sess = l.sess.add(u)
	rec := usageRecord{TS: time.Now().Format(time.RFC3339), Session: l.session,
		Kind: kind, Model: model, Usage: u}
	f, err := os.OpenFile(filepath.Join(l.dir, l.session+".jsonl"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		b, _ := json.Marshal(rec)
		f.Write(append(b, '\n'))
		f.Close()
	}
	return u
}

func (l *ledger) sessao() Usage {
	if l == nil {
		return Usage{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sess
}

// consolidar lê todos os ledgers do workspace: por sessão e o total do workspace.
func consolidar(root string) (map[string]Usage, Usage) {
	dir := filepath.Join(root, ".vitruvio", "usage")
	arqs, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	porSessao := map[string]Usage{}
	var total Usage
	for _, a := range arqs {
		b, err := os.ReadFile(a)
		if err != nil {
			continue
		}
		sid := strings.TrimSuffix(filepath.Base(a), ".jsonl")
		for _, ln := range strings.Split(string(b), "\n") {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			var r usageRecord
			if json.Unmarshal([]byte(ln), &r) != nil {
				continue
			}
			porSessao[sid] = porSessao[sid].add(r.Usage)
			total = total.add(r.Usage)
		}
	}
	return porSessao, total
}
