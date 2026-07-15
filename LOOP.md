# Vitrúvio — a disciplina do Loop

Base para usar bem o Vitrúvio. Ele **não** é uma ferramenta de *prompt → resposta*;
é um **agente em loop**: `planejar → executar → verificar → corrigir → repetir`, com
parada e rastreabilidade. Usar bem o Vitrúvio é **projetar o loop**, não caprichar no
prompt.

> Inspirado em *"Do Prompt ao Loop: A Próxima Fronteira do Desenvolvimento com IA"*,
> de **Cézar Augusto Ferreira** (2026), que nomeia esta prática de **"Loop Engineering"**.

---

## Os 5 componentes de um loop produtivo — e como o Vitrúvio entrega cada um

| Componente | No Vitrúvio | Como você aproveita |
|---|---|---|
| **1. Objetivo claro** | a sua mensagem + o `VITRUVIO.md` do projeto | descreva o *resultado*, não os passos; deixe convenções no `VITRUVIO.md` |
| **2. Verificador externo** | validação no tool-time: **Mermaid válido** e **código que compila** antes de salvar; erro volta e o agente **se auto-corrige** | confie no verificador — não é o modelo "prometendo", é um checador de verdade |
| **3. Estado persistente** | `.vitruvio/memory/` (fatos), `.vitruvio/sessions/` (histórico), `--continue` | grave decisões com `memorizar`; retome sessões |
| **4. Condição de parada** | `--max-iter` (limite de passos) e **ESC** (interromper) | pare cedo se sair do rumo; ESC cancela na hora |
| **5. Ferramentas reais** | ler/listar/criar/editar arquivos, `parse_drawio`, `gerar_diagrama`, `rodar_subagentes` | o loop **age** no projeto (com diff), não só descreve |

**Rastreabilidade** (o 6º pilar implícito): a contabilidade de **tokens** por turno/
sessão/workspace (`/usage`) e o histórico de sessão registram o que foi tentado.

---

## Como escrever um bom loop com o Vitrúvio

1. **Dê um objetivo verificável.** "Faça X passar no teste Y" > "melhore X". Quanto mais
   claro o critério de sucesso, melhor o loop converge.
2. **Prepare o ambiente.** Um `VITRUVIO.md` na raiz com stack, convenções e restrições
   entra no contexto como base (sob a constituição) — o loop respeita isso.
3. **Limite o escopo.** Peça uma mudança por vez; escopo grande = loop que se perde.
4. **Deixe o verificador trabalhar.** Diagramas e código só são salvos se passarem na
   validação; se falhar, o agente corrige sozinho. Não desligue isso à toa
   (`--validar-codigo`).
5. **Use estado.** `memorizar` decisões importantes; `--continue` para retomar.
6. **Saiba parar.** Se o loop insiste no caminho errado, **ESC** e reformule o objetivo
   (às vezes o problema é o objetivo, não o modelo).
7. **Decisão final é sua.** Revise os **diffs**; o Vitrúvio propõe, você aprova.

---

## Os riscos (do artigo) e como o Vitrúvio os mitiga

- **Dívida de compreensão** — código gerado que o time não entende (mesmo com teste
  passando) vira risco operacional. Mitigação: o Vitrúvio **sempre explica** (premissas
  → tradeoffs → alternativas → riscos) e propõe **código executável e compreensível**;
  você deve **revisar e entender** antes de aceitar. O loop não substitui compreensão.
- **Verificador fraco** — se o critério de sucesso é ruim, o loop "otimiza" a coisa
  errada com eficiência enganosa. Mitigação: verificadores **determinísticos** (compila/
  renderiza) em vez de "parece ok"; e um `eval/` com checagens objetivas (Mermaid estrito,
  compilação, recusas, red-team). **Escolha bons critérios de sucesso** — é onde mora o
  valor.

---

## Quando um loop faz sentido (com a lente do Vitrúvio)
Ambiente controlado + critério de verificação + **decisão humana final**:
- atualizar dependências e ajustar o código;
- corrigir bugs **reproduzíveis**;
- gerar/elevar testes (cobertura);
- migração (SQL Server→PostgreSQL, on-prem→AWS, containerização);
- análise de segurança (SAST/SCA) e correção;
- propor arquitetura + diagrama + tradeoffs;
- documentação técnica.

**Não** faz sentido sem critério de sucesso claro ou sem revisão humana — aí o loop
vira automação de erro.

---

## Em uma frase
O diferencial do Vitrúvio não é o modelo, é o **loop com verificação determinística e
parada**: ele **tenta, mede, corrige e para** — para que a proposta seja **confiável**,
não apenas plausível. Projete o loop; o resto ele executa.
