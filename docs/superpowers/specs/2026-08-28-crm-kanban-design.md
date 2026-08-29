# CRM — Fase 2: quadro Kanban

- **Data:** 2026-08-28
- **Status:** Aprovada — pronta para plano de implementação
- **Autor/revisão:** Ivan Coelho (product) · design colaborativo
- **Escopo:** Fase 2 de 3 do CRM. A Fase 1 (núcleo) está mergeada em `development`; a Fase 3 (SLA e relatórios) tem spec própria.

## 1. Contexto

A Fase 1 entregou ocorrências com protocolo, etapas configuráveis, timeline e as telas de lista e detalhe. A etapa muda por seletor.

Esta fase entrega o quadro: acompanhar visualmente em que ponto cada caso está, e mover arrastando.

### A decisão que orienta tudo

**Kanban é uma projeção do recurso `occurrence`, não um recurso de domínio novo.**

Não existe "quadro" no modelo. Existem ocorrências e etapas, e o quadro é uma forma de olhar para elas. A consequência prática é que **não há abstração de Kanban no backend** — nenhum endpoint `/board`, nenhum agregador, nenhuma tabela. O quadro é montado no frontend a partir do endpoint de listagem que já existe e já está testado.

## 2. Objetivos e não-objetivos

**Objetivos**

- Modo de exibição em quadro, escolhido pelo usuário, na mesma tela da lista.
- Arrastar um cartão entre colunas para mudar a etapa.
- Comportamento previsível sob volume e sob falha.

**Não-objetivos**

- **Reordenar cartões dentro da coluna.** Ver §7 — é a decisão de escopo mais importante da fase.
- **Reordenar colunas arrastando.** A ordem é o campo `position` das etapas e se muda na tela de configuração que a Fase 1 entregou.
- **SLA, prazos e cores por vencimento.** Fase 3.
- **Endpoint agregador de quadro.** Ver §1.

## 3. Backend — um parâmetro

O endpoint `GET /api/occurrences` da Fase 1 já aceita `stage_id`, `open`, `page` e `limit`, e devolve `total`. Cada coluna é uma chamada a ele com `stage_id` fixo, e ganha de graça a própria contagem para o cabeçalho.

A **única** alteração de backend da fase é um parâmetro novo: `closed_since`.

### Semântica de `closed_since`

```
closed_at IS NOT NULL AND closed_at >= <valor>
```

O limite é **inclusivo** (`>=`). Uma ocorrência fechada exatamente no instante do corte **entra**.

`closed_since` **não substitui** `open`, e os dois não são usados juntos: são regras de colunas diferentes (§5).

**Armadilha verificada no código:** o handler testa `if open == "true"`. Portanto `open=false` **não significa "só fechadas"** — significa *sem filtro nenhum*, e traria a lista inteira. A coluna de fechamento **não pode** ser montada com `open=false`; ela usa `closed_since` sozinho, que já implica fechada pela primeira metade da condição.

### Filtros do usuário

Os filtros que a tela oferece (responsável, prioridade, busca por protocolo) **se aplicam a todas as colunas, inclusive a de fechamento**, combinados com a regra da coluna. Filtrar por responsável e ver a coluna "Resolvido" ignorando o filtro seria incoerente.

## 4. Carregamento das colunas

As chamadas das colunas são **independentes e disparadas em paralelo**. O tempo de abertura do quadro é o da chamada mais lenta, não a soma delas.

**Falha é isolada por coluna.** Se uma coluna falhar, ela mostra o próprio estado de erro com opção de tentar de novo, e as demais continuam funcionando. Um quadro inteiro derrubado por uma requisição é pior que um quadro com uma coluna avisando que falhou.

Cada coluna carrega 25 e tem "carregar mais".

## 5. Quais ocorrências cada coluna mostra

Escrito de forma explícita, porque é onde uma interpretação errada do endpoint produziria o quadro errado:

| Coluna | Regra |
|---|---|
| Etapas normais (`is_closing = false`) | `stage_id=<etapa>` + `open=true` — ocorrências **abertas** daquela etapa |
| Etapas de fechamento (`is_closing = true`) | `stage_id=<etapa>` + `closed_since=<agora − 7 dias>` — fechadas nos últimos 7 dias |

Mais os filtros do usuário, em ambos os casos (§3).

**Pode haver mais de uma etapa de fechamento.** As regras de integridade da Fase 1 exigem *pelo menos uma*, não exatamente uma — uma organização pode ter "Resolvido" e "Cancelado". A regra vale para **cada** etapa com `is_closing = true`, não para uma coluna especial. O quadro não trata nenhuma coluna como singular.

(Uma etapa não pode ser inicial e de fechamento ao mesmo tempo — a Fase 1 recusa isso com 400.)

### Os 7 dias

É uma **constante do frontend** nesta fase, não configuração. Se virar preferência, é decisão futura com tela própria.

O valor enviado é um **instante absoluto** em RFC3339, calculado pelo cliente a partir do próprio relógio. O backend não interpreta "7 dias" — ele recebe uma data e compara. Isso mantém o parâmetro simples, deixa o teste em Go determinístico (basta passar uma data fixa) e evita que a semântica da janela viva em dois lugares.

A coluna de fechamento existe com conteúdo justamente para o arrastar dar retorno: soltar um cartão nela e ver o cartão chegar. Quem precisa do arquivo completo usa a lista, que tem busca e filtro.

## 6. Arrastar

Atualização **otimista**: o cartão vai para a coluna nova imediatamente e a requisição sai depois.

### Reversão

A reversão **não** depende do estado visual anterior. Ao iniciar o arrasto, guarda-se explicitamente:

```
occurrenceId, fromStageId, toStageId
```

Se a requisição falhar, o cartão volta de `toStageId` para `fromStageId` usando esses valores guardados, e aparece **a mensagem que o servidor devolveu** — não um erro genérico. Interface otimista que não desfaz é interface que mente.

### Mutações concorrentes no mesmo cartão

Um cartão com requisição em andamento **não aceita novo arrasto** até a resposta chegar. É a regra mais simples que resolve: sem ela, dois movimentos rápidos no mesmo cartão podem chegar fora de ordem e a reversão restaura o estado errado.

Nada de sistema de sincronização — apenas travar aquele cartão enquanto sua própria requisição está em voo.

### Soltar na própria coluna

Continua sendo **no-op**: não altera `closed_at` e não cria evento de timeline.

A Fase 1 já implementou e testou isso, mas num quadro deixa de ser situação excepcional — um drop acidental na mesma coluna acontece o tempo todo. Fica registrado aqui como requisito desta fase, não como detalhe herdado.

## 7. Sem ordenação manual dentro da coluna

A ordem dos cartões é a **data de abertura**, igual à lista.

Introduzir posição por ocorrência para o quadro "parecer um Kanban tradicional" arrastaria: coluna nova no banco, API nova, lógica de arrasto vertical, concorrência entre reordenações, persistência, testes, e uma decisão de produto sobre o que a ordem significa. Tudo isso para um conceito que o domínio hoje **não tem**.

Se surgir necessidade real, vira fase própria.

## 8. Frontend

O quadro **não** ganha rota própria. `/crm/occurrences` recebe um seletor lista/quadro no cabeçalho, e os filtros valem para os dois modos — é o mesmo dado noutra projeção, não outra tela.

| Arquivo | Responsabilidade |
|---|---|
| `components/crm/OccurrenceBoard.vue` | colunas, arrastar, carregar mais, erro por coluna |
| `components/crm/OccurrenceCard.vue` | o cartão |
| `composables/useOccurrenceViewMode.ts` | a preferência, em `localStorage` |

O cartão mostra protocolo, título, contato, responsável e prioridade.

A preferência de modo fica em `localStorage`, seguindo o padrão já usado por `useColorMode`, `useDateRange` e `useResizablePanel`. Modo de exibição é o tipo de preferência que faz sentido variar por dispositivo.

`vuedraggable@4.1.0` já é dependência do projeto, com peer `vue ^3.0.1` — é a versão de Vue 3 e serve. Esta fase é o primeiro uso dela no repositório; nada novo é instalado.

## 9. Verificação

**Go**

- `closed_since` filtra por `closed_at >= valor`, com **relógio fixo no teste** para o caso de borda não ficar intermitente.
- Uma ocorrência fechada **exatamente** no instante do corte entra.
- `closed_since` ignora ocorrências abertas (`closed_at IS NULL`).
- Os filtros de responsável e prioridade continuam valendo junto com `closed_since`.

**Playwright**

- Trocar para quadro, arrastar entre colunas, e conferir que a timeline registrou a mudança.
- **Falha do servidor devolve o cartão à coluna de origem**, com a mensagem do servidor. Interface otimista sem reversão testada não vale nada.
- **Depois de arrastar, a coluna continua ordenada por data de abertura.** Este teste protege a decisão de §7: se alguém introduzir ordenação manual sem spec, ele fica vermelho.
- "Carregar mais" numa coluna com mais de 25 ocorrências.
- A preferência de modo sobrevive ao recarregar.

**Teste de mutação** no filtro: remover a condição de `closed_since` deve deixar o teste de borda vermelho.

## 10. Riscos

| Risco | Mitigação |
|---|---|
| `open=false` interpretado como "só fechadas" | Documentado em §3; a coluna de fechamento usa apenas `closed_since` |
| Reversão restaurar estado errado após dois movimentos rápidos | Cartão travado enquanto sua requisição está em voo (§6) |
| Uma coluna lenta ou com erro derrubar o quadro | Chamadas paralelas e independentes, com erro isolado por coluna (§4) |
| Quadro travar com volume | Só abertas, 25 por coluna, "carregar mais" |
| Alguém introduzir `position` sem spec | O teste de ordenação por data de abertura (§9) fica vermelho |
