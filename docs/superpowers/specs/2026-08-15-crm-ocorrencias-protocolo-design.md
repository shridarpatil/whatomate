# CRM de ocorrências com protocolo de atendimento — Fase 1 (núcleo)

- **Data:** 2026-08-15
- **Status:** Aprovada — pronta para plano de implementação
- **Autor/revisão:** Ivan Coelho (product) · design colaborativo
- **Branch:** `feature/crm-ocorrencias-protocolo`
- **Escopo:** Fase 1 de 3. Kanban (Fase 2) e SLA/relatórios (Fase 3) têm specs próprios.

## 1. Contexto

O produto acompanha conversas de WhatsApp, mas não acompanha **casos**. Um cliente que abre uma reclamação, aguarda uma nota fiscal e recebe a mercadoria três semanas depois passa por várias conversas — e hoje não existe nada que amarre isso num registro único com número, etapa e histórico.

O pedido é um CRM de **ocorrências** por contato, com etapas configuráveis acompanhadas em Kanban, histórico consolidado onde hoje ficam as notas, e possibilidade de SLA. E, atravessando tudo, a **geração de protocolo de atendimento**.

### O achado que define o ponto de partida

**Atendimento não é uma entidade neste sistema.** A spec do ciclo de vida ([2026-07-21](2026-07-21-attendance-lifecycle-design.md)) registra: *"Não existe entidade Conversa"*. Um atendimento é a combinação de um `Contact` com um `AgentTransfer` ativo.

| Conceito | Onde vive hoje |
|---|---|
| Fila | `AgentTransfer` com `status='active'`, `agent_id IS NULL` |
| Atendimento em curso | `AgentTransfer.AgentID` |
| Carteira do cliente | `Contact.AssignedUserID` |
| Encerramento | `TransferStatus`: `active` → `resumed` / `expired` |
| Anotações | `ConversationNote`, presas ao contato |

E `protocol` não aparece em lugar nenhum do Go — zero ocorrências.

Isso torna a ancoragem do protocolo uma decisão de arquitetura, não um detalhe: **um número precisa estar preso a algo**, e o candidato mais próximo (`AgentTransfer`) tem três problemas. Conversa resolvida só pelo chatbot nunca cria transferência e ficaria sem protocolo; transferências encadeadas (SAC → Logística → SAC) criam várias linhas para o que o cliente entende como um atendimento só; e semanticamente é registro de *transferência*, não de atendimento.

**Ocorrência e protocolo são o mesmo objeto** — o protocolo é o número legível da ocorrência. Ela nasce como entidade nova, não sobre o `AgentTransfer`.

### Referência de mercado

O padrão consolidado (Zendesk, Freshdesk, HubSpot Service; no Brasil Blip e Zenvia) é **ticket numerado + pipeline de etapas + timeline de eventos + política de SLA**. No Brasil o protocolo carrega o peso extra de ser o número que o cliente anota e cita depois — o que impõe formato legível e busca por número.

## 2. Objetivos e não-objetivos

**Objetivos da Fase 1**

- Ocorrência por contato, com protocolo gerado automaticamente.
- Etapas configuráveis por organização, com movimentação registrada.
- Timeline consolidando notas do caso e eventos automáticos.
- Criação e acompanhamento **a partir da conversa no chat**.
- Envio do protocolo ao cliente por ação explícita do agente.

**Não-objetivos**

- **Kanban** — Fase 2. Na Fase 1 a etapa muda por seletor.
- **SLA e relatórios** — Fase 3. O modelo já nasce compatível (ver §8).
- **Vários pipelines por tipo de ocorrência.** Decisão: um conjunto de etapas por organização. Evoluir para vários exige migração das ocorrências existentes; fica registrado como custo conhecido.
- **Migrar as notas existentes.** O painel de notas do contato continua intacto (ver §5).
- **Qualquer alteração de comportamento em produção.** Ver §7.

## 3. Decisões

| Questão | Decisão | Motivo |
|---|---|---|
| Ocorrência × conversa | Independentes, com vínculo de origem | Um contato pode ter vários casos abertos; acoplar ao ciclo de vida atual seria o oposto de aditivo |
| Formato do protocolo | `2026-000123` — ano + sequencial por org | Curto de ditar, ordena sozinho, o ano tria "protocolo velho" na hora |
| Notas | Dois níveis, com rótulos claros | Nota de contato é sobre a **pessoa**; timeline é sobre o **caso**. É a separação de Zendesk e HubSpot |
| Etapas | Um conjunto configurável por organização | Atende o pedido sem a tela de configuração maior de múltiplos pipelines |
| Visibilidade | Herda de `authorizeConversation` + responsável sempre vê | Regra própria viraria caminho alternativo para ver conversas escondidas |
| Envio do protocolo | Botão explícito | Nenhuma mensagem paga sai sozinha; fora da janela de 24h só template é aceito |

## 4. Modelo de dados

Quatro tabelas novas. **Nenhuma coluna adicionada a tabela existente.**

### `occurrences`

| Campo | Tipo | Observação |
|---|---|---|
| `organization_id` | uuid | |
| `contact_id` | uuid | |
| `protocol_number` | string(20) | `"2026-000123"`, índice único junto com `organization_id` |
| `title` | string(255) | |
| `description` | text | |
| `stage_id` | uuid | FK para `occurrence_stages` |
| `priority` | string(20) | `low` / `normal` / `high` / `urgent`, padrão `normal` |
| `assigned_user_id` | uuid, nulo | responsável pelo caso |
| `team_id` | uuid, nulo | |
| `opened_by_user_id` | uuid | |
| `opened_at` | timestamp | |
| `closed_at` | timestamp, nulo | preenchido ao entrar em etapa `is_closing` |
| `source_transfer_id` | uuid, nulo | atendimento que originou; nulo quando criada fora de uma conversa |

`source_transfer_id` é o que materializa "independentes, mas ligadas": dá o caminho de volta da ocorrência para a conversa de origem **sem** acoplar os ciclos de vida. Encerrar o atendimento no chat não fecha a ocorrência, e vice-versa.

`protocol_number` ordena lexicograficamente na ordem correta por causa do zero-padding, então ano e sequência não precisam de colunas próprias.

### `occurrence_stages`

`organization_id`, `name`, `color`, `position`, `is_initial`, `is_closing`.

Semeadas na migração inicial, por organização: **Aberto** (`is_initial`) → **Em análise** → **Aguardando cliente** → **Resolvido** (`is_closing`).

Regras: exatamente uma etapa `is_initial`; pelo menos uma `is_closing`; excluir etapa só é permitido quando nenhuma ocorrência a referencia.

### `occurrence_events`

`organization_id`, `occurrence_id`, `type`, `content`, `metadata` (jsonb), `created_by_id` (nulo = sistema).

Tipos: `opened`, `note`, `stage_change`, `assignment`, `protocol_sent`, `closed`.

Notas manuais e eventos automáticos na mesma tabela. Duas tabelas separadas divergem com o tempo e obrigam a intercalar duas consultas para montar uma linha do tempo.

### `occurrence_counters`

`organization_id`, `year`, `last_seq` — chave primária composta por `(organization_id, year)`.

A numeração usa `UPDATE occurrence_counters SET last_seq = last_seq + 1 WHERE organization_id = ? AND year = ? RETURNING last_seq`, com upsert para o primeiro do ano, **dentro da mesma transação do insert da ocorrência**. O `UPDATE ... RETURNING` é atômico no Postgres: duas ocorrências abertas no mesmo instante recebem sequências diferentes. Protocolo repetido é pior que protocolo feio — é por isso que a numeração não sai de `COUNT(*) + 1`.

## 5. Notas: dois níveis

O painel `ConversationNotes.vue` e os endpoints `/api/contacts/{id}/notes` **continuam exatamente como estão**. Nada é migrado, nada é desativado.

A separação fica explícita nos rótulos da interface:

- **Notas do contato** — sobre a pessoa. "Prefere ser chamado de Zé", "cliente desde 2019".
- **Timeline da ocorrência** — sobre o caso. "Aguardando NF do fornecedor", "cliente confirmou o endereço".

## 6. Visibilidade

Todo handler passa por `canViewConversation` / `canInteractWithConversation` ([conversation_visibility.go](../../../internal/handlers/conversation_visibility.go)) usando o contato da ocorrência. A listagem aplica `scopeVisibleConversations` num join com `contacts`, unido por `OR` com `assigned_user_id = <usuário atual>`.

Essa exceção é deliberada: sem ela, atribuir uma ocorrência a alguém que não enxerga aquele contato produz um caso invisível para o próprio responsável. O alcance é estreito — a pessoa vê a ocorrência que lhe foi atribuída, não a lista de conversas do contato.

Permissões novas: `occurrences:read`, `occurrences:write`, `occurrence_stages:write`.

## 7. Por que nada em produção muda

| Superfície | Garantia |
|---|---|
| Tabelas existentes | Nenhuma coluna adicionada a `contacts`, `agent_transfers`, `messages` |
| Fluxo do chatbot | Não tocado |
| Processador de SLA | Não tocado |
| Ciclo de transferência | Não tocado |
| Painel de notas | Idêntico |
| Envio de mensagem | Reusa `SendOutgoingMessage` ([messages.go:148](../../../internal/handlers/messages.go)) sem modificá-lo |
| Visibilidade | Nenhuma regra nova; reusa o autorizador existente |

Tudo é aditivo: tabelas novas, endpoints novos, telas novas.

## 8. Compatibilidade com as fases seguintes

**Fase 2 (Kanban):** as colunas do quadro são as `occurrence_stages` ordenadas por `position`; arrastar chama o mesmo `PUT /stage` da Fase 1. `vuedraggable` já é dependência do projeto — não entra biblioteca nova.

**Fase 3 (SLA):** `SLATracking` ([chatbot.go:290](../../../internal/models/chatbot.go)) é struct **embutida**, então passa a ser embutida em `occurrences` sem alterar o que já existe, e o motor de `sla_processor.go` é reaproveitado.

## 9. API

```
GET    /api/occurrences                       lista, filtros: stage, assigned, contact, protocol, status
POST   /api/occurrences                       cria e gera o protocolo
GET    /api/occurrences/{id}
PUT    /api/occurrences/{id}                  título, descrição, prioridade, responsável
PUT    /api/occurrences/{id}/stage            move de etapa; grava evento stage_change
POST   /api/occurrences/{id}/events           adiciona nota à timeline
POST   /api/occurrences/{id}/send-protocol    envia o número ao cliente
GET    /api/contacts/{id}/occurrences         alimenta o painel do chat

GET    /api/occurrence-stages
POST   /api/occurrence-stages
PUT    /api/occurrence-stages/{id}
DELETE /api/occurrence-stages/{id}
PUT    /api/occurrence-stages/reorder
```

## 10. Frontend

| Arquivo | Responsabilidade |
|---|---|
| `views/crm/OccurrencesView.vue` | Lista com filtros e busca por protocolo |
| `views/crm/OccurrenceDetailView.vue` | Detalhe, seletor de etapa, timeline |
| `components/chat/ContactOccurrencesPanel.vue` | Painel no chat: ocorrências do contato + criar |
| `views/settings/OccurrenceStagesView.vue` | Configuração das etapas |
| `stores/occurrences.ts` | Estado |

Chaves de i18n em `pt-BR.json` **e** `en.json`, mantidos paralelos linha a linha.

O painel no chat é irmão do de notas: lista protocolo e etapa de cada ocorrência do contato, com botão "nova ocorrência" já preenchido com o contato e com `source_transfer_id` do atendimento corrente.

O botão "enviar protocolo" fica **desabilitado, com aviso**, quando `last_inbound_at` passou de 24h — fora da janela só template é aceito, e um botão que falha silenciosamente é pior que um botão desabilitado.

## 11. Verificação

**Go**

- Numeração de protocolo **sob concorrência** — o teste que mais importa. Goroutines paralelas criando ocorrências na mesma organização e ano, asserindo unicidade. Sem ele, a atomicidade do `UPDATE ... RETURNING` é só uma afirmação.
- Virada de ano: primeira ocorrência de um ano novo começa em `000001`.
- Transições de etapa: entrar em `is_closing` preenche `closed_at`; sair limpa.
- Gates de visibilidade com casos **positivos e negativos**. Este código já teve quatro rodadas de correção de vazamento de visibilidade — teste negativo é obrigatório, não opcional.
- A exceção do responsável: quem tem a ocorrência atribuída a enxerga mesmo sem ver o contato.

**Playwright**

Criar ocorrência pela conversa, mover etapa, anotar na timeline, enviar protocolo. E o estado desabilitado do botão fora da janela de 24h.

**Teste de mutação** nos pontos de decisão (numeração, gate de visibilidade), como foi feito na identidade do 9º dígito: desativar a regra e confirmar que a suíte fica vermelha. Teste que passa com a lógica removida não é teste.

**Ambiente:** Postgres e Redis em Docker; backend em `:8080`; Vite em `:3000` com recarga automática.

## 12. Riscos conhecidos

| Risco | Mitigação |
|---|---|
| Numeração concorrente gerar protocolo duplicado | `UPDATE ... RETURNING` na mesma transação + índice único `(organization_id, protocol_number)` + teste de concorrência |
| CRM virar caminho alternativo para ver conversas escondidas | Reuso obrigatório de `authorizeConversation`; testes negativos |
| Evoluir para múltiplos pipelines depois | Custo aceito e registrado: exigirá migração das ocorrências existentes |
| Dois lugares para escrever anotação confundir o agente | Rótulos explícitos na interface separando pessoa e caso |
