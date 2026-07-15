# Vitrúvio CLI

Cliente agêntico do **Vitrúvio** (arquitetura + segurança), estilo Claude Code:
TUI de tela cheia, lê/cria/edita arquivos, memória de projeto, contabilidade de
tokens e subagentes. Também roda como **servidor** (inferência + guardrails) para
a extensão do VS Code.

> **Arquitetura C (client-centric):** o servidor faz **inferência pura + guardrails**
> (compõe a constituição e a repassa à engine); o **cliente** (esta CLI ou a extensão)
> roda o loop do agente e executa as ferramentas. O servidor **não** acessa o disco.

> 📖 **Antes de usar, leia [LOOP.md](LOOP.md)** — a disciplina do loop
> (planejar→executar→verificar→corrigir→parar) é a base para tirar proveito do Vitrúvio.

O binário **não** é versionado — você compila na máquina.

---

## 1. Compilar

```bash
cd v2/cli
go build -o vitruvio.exe .    # Windows
# go build -o vitruvio .       # Linux/macOS
```

A 1ª compilação baixa as dependências Go (`bubbletea`, `lipgloss`, `x/term`, …).
A máquina precisa de acesso a módulos Go **uma vez** (ou use `go mod vendor`).

---

## 2. Subir uma engine (obrigatório)

O Vitrúvio é **agnóstico de engine** — ele fala OpenAI `/v1/chat/completions`.
Escolha **uma**:

### Opção A — Ollama (dev, mais simples)
```bash
ollama serve                        # sobe em http://localhost:11434
ollama pull qwen2.5:7b-instruct     # ou o modelo que preferir
```
Padrões do CLI já apontam para o Ollama (`--host http://localhost:11434`).

### Opção B — Nosso modelo próprio (GGUF via llama.cpp)
Sem Ollama, rodando o **nosso** modelo (ou o Qwen quantizado) direto no llama.cpp:
```bash
# 1) compile o llama.cpp na máquina (compile o llama.cpp: https://github.com/ggml-org/llama.cpp)
# 2) suba o servidor da engine com o .gguf
llama-server -m vitruvio.gguf --host 127.0.0.1 --port 8080
```
Aí o CLI aponta para essa porta com `--host http://localhost:8080`
(nesse caso o nome do `--model` é ignorado — a engine já carregou o modelo).

---

## 3. Modo servidor (para a extensão do VS Code)

Sobe o endpoint OpenAI-compat **com os guardrails** que a extensão consome:

```bash
# com Ollama:
vitruvio --serve --port 8090 --model qwen2.5:7b-instruct

# com o modelo próprio (llama.cpp na 8080):
vitruvio --serve --port 8090 --host http://localhost:8080
```

Fluxo: **extensão → `vitruvio --serve` (constituição/guardrails) → engine**.

Endpoints expostos:

| Endpoint | O que faz |
|---|---|
| `POST /v1/chat/completions` | 1 rodada; injeta a constituição, ignora `system` do cliente, devolve `tool_calls` + `usage` |
| `GET /v1/models` | lista modelos da engine |
| `GET /v1/agents` | skills embutidas |
| `POST /v1/tools/parse_drawio` | decodifica draw.io (compute puro) |
| `GET /v1/usage` | consumo de tokens (sessão + workspace) |

Na extensão, configure `vitruvio.endpoint` para `http://localhost:8090/v1/chat/completions`.

---

## 4. Modo interativo (TUI) e outros usos

```bash
vitruvio                                   # TUI (precisa de uma engine no ar)
vitruvio --host http://localhost:8080      # usando o modelo próprio
vitruvio "analise infra.drawio e aponte gaps de segurança"   # one-shot
vitruvio --continue                        # retoma a última sessão do workspace
vitruvio --usage                           # consumo de tokens (sessão/workspace)
vitruvio --list-skills                     # lista skills/agentes
vitruvio --skill escrever-adr "contexto…"  # roda uma skill
```

**Comandos no TUI** (digite `/`): `/ajuda` `/usage` `/memoria` `/skills`
`/compactar` `/limpar` `/continuar` `/sair`. **ESC** interrompe o agente;
**Ctrl+C** sai.

---

## 5. Flags

| Flag | Padrão | Descrição |
|---|---|---|
| `--host` | `http://localhost:11434` | endpoint da engine (Ollama ou llama-server) |
| `--model` | `qwen2.5:7b-instruct` | modelo (ignorado no llama-server) |
| `--dir` | `.` | diretório de trabalho (raiz das ferramentas) |
| `--serve` | `false` | modo servidor (API para a extensão) |
| `--port` | `8090` | porta do modo servidor |
| `--max-iter` | `12` | máx. de passos do agente por turno |
| `--skill` / `--skills-dir` / `--list-skills` | — | skills |
| `--continue` | `false` | retoma a última sessão |
| `--usage` | `false` | mostra o consumo e sai |
| `--no-color` | `false` | desativa cores (ou use `NO_COLOR=1`) |

---

## 6. Memória, contexto e tokens (no workspace)

- `VITRUVIO.md` na raiz → contexto/convenções do projeto (entra no prompt como
  **menor autoridade**, sempre sob a constituição).
- `.vitruvio/memory/` → memória persistente (a ferramenta `memorizar` grava aqui).
- `.vitruvio/sessions/` → histórico das sessões (`--continue`).
- `.vitruvio/usage/` → ledger de tokens por sessão (consolidado em `--usage`).
- `.vitruvio/agents/` e `.vitruvio/skills/` → agentes/skills do workspace.

Esses diretórios são **do runtime** (ficam fora do git).

---

## 7. Notas

- **Terminal:** use o **Windows Terminal** (não o `cmd.exe` clássico) para a TUI e os
  blocos do mascote renderizarem certo.
- **CPU-only:** num notebook (ex.: Ryzen 7 7730U) rode um **GGUF quantizado (Q4_K_M)**
  no llama.cpp; a inferência é por CPU (a iGPU não acelera de forma relevante).
- **Guardrails:** a constituição é composta **no servidor** e tem precedência sobre
  instruções de usuário/agente/workspace — memória e `VITRUVIO.md` não a sobrepõem.

---

## Licença

Apache-2.0 © 2026 Pedro Lima. Derivado do Qwen2.5-Coder (Apache-2.0). Parte do projeto **Vitrúvio**.
