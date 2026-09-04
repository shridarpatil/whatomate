# CRM: PUT de descrição sem full-replace, e tempo real por WebSocket

- **Data:** 2026-09-03
- **Status:** Aprovada — pronta para plano de implementação
- **Autor/revisão:** Ivan Coelho (product) · design colaborativo
- **Escopo:** duas correções pequenas e independentes no CRM de ocorrências, ambas registradas como pendência desde a Fase 1. Nenhuma mexe em visibilidade, permissão ou no desenho do quadro Kanban.

## 1. O PUT que apaga a descrição

### O problema

`UpdateOccurrenceRequest.Description` é `string` simples ([occurrences.go:294](../../../internal/handlers/occurrences.go)). O handler grava `updates["description"] = req.Description` incondicionalmente. Um cliente que faz `PUT` enviando só os campos que quer mudar — omitindo `description` porque não quer alterá-la — recebe de volta uma ocorrência com a descrição apagada, porque o JSON omitido decodifica para `""`, indistinguível de "o cliente quer apagar".

O mesmo arquivo já resolveu esse problema exato para `AssignedUserID`, com `*string` e o comentário em [occurrences.go:406-410](../../../internal/handlers/occurrences.go):

> `req.AssignedUserID != nil` é o que diz que o campo participou da requisição. Um PUT que omite o campo deixa o responsável intocado — só um PUT que envia explicitamente `""` o limpa.

### A decisão

`Description` vira `*string`, seguindo exatamente o mesmo padrão:

| Corpo da requisição | Efeito |
|---|---|
| campo `description` ausente | descrição não muda |
| `"description": ""` | descrição é apagada |
| `"description": "texto"` | descrição vira "texto" |

**Contrato preservado para quem já envia o campo preenchido.** Um cliente que sempre manda `description` com o valor atual (o comportamento de qualquer formulário que carrega o registro antes de editar) não percebe diferença nenhuma — o valor que ele manda é o valor que fica.

Mudança de uma linha no bloco `updates`, mais o teste que faltava: `PUT` sem o campo `description` preserva o valor existente.

## 2. Tempo real no CRM

### O problema

Nenhum handler de ocorrência emite evento de WebSocket. Dois agentes olhando o mesmo quadro Kanban não veem a movimentação um do outro — só descobrem ao recarregar a página. O mecanismo de broadcast já existe e está testado (`internal/websocket/hub.go`), usado hoje só por notas de conversa.

### Por que `BroadcastToOrg`, não `BroadcastToContact`

`BroadcastToContact` só entrega a clientes que enviaram `set_contact` explicitamente — mecanismo específico da tela de chat ([client.go:261-279](../../../internal/websocket/client.go)). A tela de detalhe de ocorrência (`/crm/occurrences/:id`) não é a tela de chat e não participa desse protocolo. Tentar reaproveitar `BroadcastToContact` exigiria ensinar a tela de detalhe a fingir que "selecionou" o contato — acoplamento a uma semântica que não é a dela.

O quadro Kanban, por desenho, é uma visão da organização inteira, não de um contato específico. `BroadcastToOrg` é o mecanismo certo para os dois eventos: entrega para todo mundo autenticado na organização, sem filtro de audiência no servidor, e o frontend decide o que é relevante — exatamente o mesmo padrão que `BroadcastToOrg` já emprega para outros eventos administrativos no sistema.

### `occurrence_changed`

**Payload:** a ocorrência inteira, no mesmo formato que a API REST já devolve (`occurrenceToResponse`) — sem payload novo para manter, sem risco de os dois formatos divergirem com o tempo.

**Emitido em:** `CreateOccurrence`, `UpdateOccurrence`, `ChangeOccurrenceStage` — **depois** de a persistência ter sido confirmada com sucesso, nunca antes. Um cliente jamais recebe um estado que acabou não sendo commitado; se o `Create`/`Update` falhar, não há broadcast.

**Entrega:** `BroadcastToOrg`, sempre.

**No quadro:**

- Se a ocorrência já é conhecida numa coluna carregada, a store substitui os dados no lugar.
- Se a etapa mudou em relação ao que a store tinha registrado, remove da coluna antiga e insere na nova, mantendo a ordenação por `opened_at` — o mesmo invariante que o arrastar-e-soltar já garante depois de um movimento bem-sucedido.
- **Regra específica para `CreateOccurrence`:** se a ocorrência nova pertence a uma coluna atualmente carregada mas ainda não foi vista pela store (é a primeira notícia dela), o card **não** entra na lista imediatamente — só o total do cabeçalho da coluna é incrementado. É o mesmo comportamento que o quadro já tem quando uma contagem muda antes de o card em si ter sido buscado (paginação), e evita duas formas de o mesmo dado aparecer: um card inserido fora da ordem de paginação normal, e a contagem batendo com uma lista que não bate com ela.

**No detalhe:** se o `occurrence_id` do payload corresponde à ocorrência que está na rota atual, os campos da tela são atualizados. Caso contrário, o evento é ignorado.

**Na lista (`OccurrencesView`):** nada. Não recebe este evento nem nenhum outro. O problema descrito é especificamente sobre o quadro e o detalhe; a lista continua exatamente como está, atualizando ao navegar.

### `occurrence_event_created`

**Payload:** mesmo formato de `conversation_note_created` — o evento de timeline como a API já o representa.

**Emitido em dois pontos**, não um — verificado contra o código, não assumido: `CreateOccurrenceEvent` grava a nota manual, mas `ChangeOccurrenceStage` grava o evento automático de mudança de etapa com um `a.DB.Create(&models.OccurrenceEvent{...})` próprio, direto, sem passar por `CreateOccurrenceEvent` ([occurrences.go:503](../../../internal/handlers/occurrences.go)). São dois `Create` diferentes na mesma tabela; o broadcast entra logo depois de cada um.

> **Correção registrada.** Uma versão anterior desta spec afirmava que `CreateOccurrenceEvent` sozinho cobria os dois casos. Falso — conferido lendo `ChangeOccurrenceStage` linha a linha antes de escrever o plano.

**Sem DTO próprio de WebSocket.** O payload reaproveita a mesma struct de resposta que `conversation_note_created` já usa para evento de timeline — não se cria uma forma nova só para ocorrências. Isso vale tanto para o evento manual quanto para o automático de mudança de etapa: os dois usam exatamente o mesmo formato de payload, porque os dois são a mesma tabela (`OccurrenceEvent`) representada da mesma forma.

**`ChangeOccurrenceStage` emite dois eventos distintos, não um.** Quando uma mudança de etapa é bem-sucedida, o handler grava duas coisas — a ocorrência atualizada (`stage_id`, `closed_at` se aplicável) e o evento automático de timeline — e cada gravação tem seu próprio broadcast: um `occurrence_changed` e um `occurrence_event_created`. O teste de "exatamente um" em §4 é por tipo de evento, não "exatamente um evento de WebSocket no total" — `ChangeOccurrenceStage` sozinho produz dois.

**Entrega:** `BroadcastToOrg`.

**No detalhe:** processa apenas quando `occurrence_id` do evento corresponde à ocorrência aberta na tela; caso contrário, ignora. O quadro não faz nada com este evento — a timeline não aparece no card.

## 3. Não-objetivos

- Nenhuma mudança em `visibleOccurrences`, `resolveAssignee`, `loadAuthorizedOccurrence` ou qualquer lógica de visibilidade e permissão.
- Nenhum broadcast novo em `OccurrencesView` (a tela de lista).
- Nenhuma mudança no protocolo `set_contact` nem na semântica de `BroadcastToContact`.
- Nenhum campo novo de payload além do que a API REST já expõe.

## 4. Verificação

**Go**

- `PUT` sem `description` no corpo preserva o valor existente.
- `PUT` com `"description": ""` apaga.
- `PUT` com `description` preenchido grava o novo valor (regressão do comportamento atual).
- `CreateOccurrence` dispara exatamente um `occurrence_changed` via `BroadcastToOrg`; o payload é conferido campo a campo contra o estado persistido — em particular `occurrence_id` e `stage_id`, porque são os dois campos que o quadro usa para decidir entre atualizar, mover de coluna ou só incrementar o contador. Não basta o evento existir; o teste falha se `occurrence_id` ou `stage_id` não baterem com a linha gravada.
- `UpdateOccurrence` dispara exatamente um `occurrence_changed`.
- `ChangeOccurrenceStage` dispara **exatamente um `occurrence_changed` e exatamente um `occurrence_event_created`** — dois eventos, de dois tipos diferentes, não um evento genérico contado uma vez. O teste verifica os dois tipos separadamente; contar "quantidade total de mensagens de WebSocket" não seria a asserção certa aqui.
- Nenhum dos três broadcasts acontece se a operação falhar antes da persistência (erro de validação, conflito, etc.) — testado forçando uma falha e confirmando zero mensagens no canal.
- `CreateOccurrenceEvent` (nota manual) dispara `occurrence_event_created` com o mesmo formato de payload que `conversation_note_created` usa.

**Playwright**

- Dois clientes autenticados, ambos com o quadro aberto: um move um card de coluna, o outro vê o card se mover sem recarregar.
- Um cliente com o quadro aberto, a ocorrência criada por outro cliente em uma coluna carregada: o total da coluna sobe, o card não aparece até a próxima carga/paginação.
- Um cliente com a tela de detalhe de uma ocorrência aberta, outro cliente muda o título dela: o campo atualiza na tela do primeiro sem recarregar.
- Um cliente na tela de detalhe da ocorrência A, outro cliente adiciona uma nota na ocorrência B: nada muda na tela do primeiro.
- Um cliente na tela de lista (`OccurrencesView`): nenhuma mudança de comportamento, nenhum evento processado.

## 5. Riscos

| Risco | Mitigação |
|---|---|
| `PUT` com `description` sempre preenchido (uso normal) mudar de comportamento | Contrato preservado por desenho; teste de regressão dedicado |
| Card inserido fora de ordem de paginação | Regra explícita: só incrementa total até a próxima carga da coluna |
| Detalhe processando evento de outra ocorrência | Comparação de `occurrence_id` antes de qualquer mutação de estado |
