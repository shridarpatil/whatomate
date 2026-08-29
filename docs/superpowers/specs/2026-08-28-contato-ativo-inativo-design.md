# Contato ativo/inativo, com histórico preservado

- **Data:** 2026-08-28
- **Status:** Aprovada — pronta para plano de implementação
- **Autor/revisão:** Ivan Coelho (product) · design colaborativo
- **Escopo:** Substituir a exclusão de contato por inativação reversível. A exclusão/anonimização definitiva (LGPD) tem spec própria.

## 1. Contexto

Ao validar a Fase 1 do CRM, uma pergunta expôs um problema mais antigo: um contato apagado **órfã suas ocorrências e conversas**. O detalhe devolve 404, a listagem esconde, e o histórico some.

Isso contradiz a premissa que justifica o índice único total do `protocol_number`: um protocolo apagado nunca pode ser reemitido *porque o cliente tem o número anotado*. De que serve o número guardado se o atendente não acha nada quando o cliente liga citando?

### O diagnóstico

**"Excluir contato" hoje é soft delete.** `DeleteContact` ([contacts.go:1588](../../../internal/handlers/contacts.go)) faz `DB.Delete(contact)`. O GORM então escopa `deleted_at IS NULL` automaticamente em toda consulta, e o registro desaparece de tudo.

**A reativação que queremos já existe.** `GetOrCreateContact` ([contactutil.go:120](../../../internal/contactutil/contactutil.go)) restaura o contato soft-deleted quando chega mensagem nova. O cliente voltar a escrever **já** reaproveita o cadastro em vez de criar outro.

Ou seja: o comportamento desejado está implementado. O que está errado é a **semântica** — o campo se chama `deleted_at`, o ORM o trata como exclusão, e o histórico é escondido junto. Não estamos construindo reativação; estamos separando dois conceitos que hoje compartilham um campo.

**`Contact` não tem conceito de ativo/inativo.** Nenhum campo equivalente existe no modelo.

## 2. Objetivos e não-objetivos

**Objetivos**

- Contato ativo/inativo, com o inativo saindo de circulação mas mantendo todo o histórico acessível.
- Reativação automática quando o cliente volta a escrever, sem quebrar a continuidade.
- Aviso explícito ao agente de que aquele contato foi reativado, e por que havia sido inativado.
- Histórico repetível: um contato pode passar por ativo → inativo → ativo → inativo várias vezes.

**Não-objetivos**

- **Exclusão e anonimização definitiva (LGPD).** Spec própria. Anonimizar de verdade toca mensagens, mídia e logs de auditoria, e envolve decidir o que fazer com o protocolo — é mais trabalho que toda a inativação. Enquanto isso, **ninguém consegue apagar um contato pela interface**, o que é mais seguro que hoje.
- Bloqueio de contato (impedir mensagens de entrar). Inativar não é banir.
- Motivos padronizados por lista. O motivo é texto livre obrigatório; catalogá-lo é decisão futura.

## 3. Decisões

| Questão | Decisão | Motivo |
|---|---|---|
| O botão "Excluir" | Vira **"Inativar"** | Elimina o caminho que órfã histórico; `deleted_at` fica reservado para LGPD |
| Reativação por mensagem | **Automática** | É o que o código já faz; ninguém perde mensagem por cair em cadastro inativo |
| Contatos já apagados | **Migram para inativos** | Eles já voltavam sozinhos na próxima mensagem — a migração só para de esconder o que ia reaparecer |
| LGPD | **Fase própria** | Escopo e risco muito maiores que a inativação |
| Motivo da inativação | **Obrigatório** | Sem motivo, o aviso diz "foi reativado" e o agente segue sem contexto |
| Aviso de reativação | **Até alguém dispensar** | Garante que alguém viu, mesmo com a conversa passando de mão em mão |
| Onde o inativo some | Lista de contatos (com filtro), lista de conversas, busca e seletores, **campanhas e envio em massa** | O último é o de maior consequência: marketing pago para quem foi tirado de circulação |

## 4. Modelo de dados

### `contacts` — uma coluna

`is_active bool`, padrão `true`, indexado. **Só isso.**

A alternativa descartada era um conjunto de colunas (`deactivated_at`, `deactivated_by`, `deactivation_reason`, `reactivated_at`, `reactivated_by`). Ela degrada rápido quando o mesmo contato percorre ativo → inativo → ativo → inativo: as colunas guardam apenas o último ciclo e o anterior se perde. E `contacts` é tabela quente, com mais de 12 mil linhas em produção.

### `contact_status_events` — tabela nova

| Campo | Observação |
|---|---|
| `organization_id` | |
| `contact_id` | |
| `type` | `deactivated` \| `reactivated` |
| `reason` | texto. Obrigatório na inativação manual; `incoming_message` na reativação automática |
| `created_by_id` | **nulo quando é o sistema** |
| `metadata` | jsonb, para contexto de auditoria |
| `acknowledged_at` | nulo até um agente dispensar o aviso; só significativo em `reactivated` |

`created_by_id` nulo é o que torna esta tabela viável e `ConversationNote` inviável: lá o campo é `not null` com chave estrangeira para usuários ([conversation_notes.go:12](../../../internal/models/conversation_notes.go)), então uma nota do sistema exigiria tornar a coluna nulável numa tabela de produção ou inventar um usuário falso.

O formato segue `occurrence_events`, estabelecido na Fase 1 do CRM — mesma ideia de linha do tempo com autor opcional.

## 5. Reativação automática: idempotência

Este é o ponto de maior risco técnico, e a lição é a mesma da numeração do protocolo: **duas mensagens simultâneas não podem gerar dois eventos `reactivated` para a mesma transição.**

A regra é que o evento só existe quando houve **efetivamente** uma transição `false → true`. Uma leitura seguida de escrita não garante isso — duas goroutines podem ler `is_active = false` e ambas gravar.

A operação é uma atualização condicional, dentro da mesma transação do evento:

```sql
UPDATE contacts SET is_active = true
WHERE id = ? AND organization_id = ? AND is_active = false
```

Só quem obtiver `RowsAffected == 1` registra o evento. O Postgres serializa as concorrentes na linha, e a perdedora enxerga `is_active = true` e não afeta nenhuma linha. Mensagem de contato **já ativo** não afeta linha nenhuma e portanto não gera evento — que é o comportamento correto.

O evento gerado tem `type = 'reactivated'`, `created_by_id = NULL` e `reason = 'incoming_message'`, deixando explícito que foi automático e o que o disparou.

**Consequência importante do desenho, registrada aqui para não surpreender:** reativação automática por mensagem nova **gera banner**. É o caso mais comum, não a exceção.

### Reativação manual

Reativar pelo filtro "mostrar inativos" também grava um evento `reactivated`, mas com `created_by_id` preenchido e `acknowledged_at` **já carimbado no momento da criação**.

Quem clicou em reativar sabe que reativou; mostrar um banner avisando disso seria ruído. O evento existe para o histórico, não para o aviso.

## 6. O aviso ao agente

**Qual evento o banner acompanha:** a **última** reativação não reconhecida — o evento mais recente com `type = 'reactivated'` e `acknowledged_at IS NULL`. Não "algum evento sem reconhecimento": se houver mais de uma reativação, é a última que vale.

O banner segue o padrão do aviso de janela de 24h que já existe no chat ([ChatView.vue:2596](../../../frontend/src/views/chat/ChatView.vue)), e mostra: quando foi inativado, por quem, e o motivo — lidos do evento `deactivated` anterior.

**Dispensar** carimba `acknowledged_at` **naquele evento específico**, cujo id o banner carrega. Nunca "o evento mais recente no momento do clique", que abriria janela para reconhecer o evento errado se outra reativação acontecer no intervalo.

O evento **não é apagado**. `acknowledged_at` separa duas coisas distintas: *isso aconteceu* e *o agente tomou conhecimento*. O histórico completo permanece visível no painel do contato.

## 7. Onde o inativo some

| Superfície | Comportamento |
|---|---|
| Lista de contatos | Escondido, com filtro "mostrar inativos" para encontrar e reativar à mão |
| Lista de conversas do chat | Escondido |
| Busca e seletores de contato | Não aparece |
| **Campanhas e envio em massa** | **Não entra** |

Sem o filtro na lista de contatos, inativar vira caminho sem volta pela interface.

### O que inativar NÃO faz

Inativar mexe no **cadastro**, não no atendimento em curso.

- **Não encerra atendimento aberto** nem mexe em transferência ativa. Se houver conversa em andamento, ela segue — encerrá-la é decisão de quem atende, por outro caminho.
- **Não bloqueia envio numa conversa já aberta.** Um agente com a conversa na tela continua conseguindo responder. Interromper um envio no meio de um atendimento seria surpresa ruim, e inativar não é banir (ver §2).
- **Não impede a mensagem do cliente de chegar.** Ela chega e reativa o cadastro (§5).

O efeito de inativar é sair das listagens e das seleções — não perder a capacidade de conversar com quem está falando com você agora.

Os pontos exatos onde campanhas selecionam destinatários **serão auditados no plano**, não presumidos. É a superfície de maior consequência: disparar mensagem paga para quem foi tirado de circulação custa dinheiro e pode ser exatamente o motivo da inativação.

## 8. Migração

`deleted_at IS NOT NULL` → `is_active = false`, `deleted_at` limpo, mais um evento `deactivated`.

O evento carrega em `metadata` o valor original de `deleted_at` e a marcação de que veio de migração. Isso preserva a informação anterior para auditoria e torna a operação reversível na prática: dá para reconstruir o estado original a partir dos eventos.

**Pré-condição de rollout, não verificação posterior:** antes de executar a migração em produção, uma consulta somente leitura precisa informar **quantos contatos serão afetados**, e esse número precisa ser revisado e aprovado. Um número inesperadamente alto significa que a premissa está errada e a migração não deve rodar.

A migração não expõe nada novo: esses contatos já reapareciam sozinhos na próxima mensagem do cliente, apenas sem o histórico ligado.

## 9. Por que isso é diferente da Fase 1 do CRM

A Fase 1 foi **puramente aditiva** — tabelas novas, endpoints novos, telas novas, nenhum comportamento existente alterado.

**Esta fase não é.** O botão de excluir muda de significado, quatro listagens ganham filtro, e a migração toca dados de produção. O rollout exige mais cuidado:

- a contagem de afetados é pré-condição (§8);
- a mudança de "Excluir" para "Inativar" precisa ser comunicada a quem opera, porque o botão que eles conhecem passa a fazer outra coisa — mais segura, mas diferente;
- os eventos de migração tornam o passo reversível na prática.

## 10. Verificação

**Go**

- Inativar **exige** motivo; sem motivo, recusa.
- Inativar esconde das quatro superfícies, com teste por superfície.
- **Idempotência da reativação sob concorrência** — o teste que mais importa. Goroutines paralelas processando mensagens do mesmo contato inativo, asserindo **exatamente um** evento `reactivated`. Sem ele, a atomicidade da atualização condicional é só uma afirmação.
- Mensagem de contato **já ativo** não gera evento.
- Dispensar carimba `acknowledged_at` no evento certo e não apaga nada.
- O banner acompanha a **última** reativação não reconhecida, com teste que cria duas reativações.
- A migração converte corretamente e preserva `deleted_at` original no `metadata`.

**Teste de mutação** nos dois pontos de decisão: remover a condição `AND is_active = false` da atualização deve deixar o teste de concorrência vermelho; remover o filtro de campanhas deve deixar o teste de campanha vermelho. Teste que passa com a lógica removida não é teste.

**Playwright:** inativar com motivo, sumir da lista, aparecer com o filtro, cliente escreve, banner aparece com o motivo, dispensar, banner some e histórico permanece.

## 11. Riscos

| Risco | Mitigação |
|---|---|
| Reativação concorrente gerar eventos duplicados | Atualização condicional com `RowsAffected` na mesma transação + teste de concorrência + teste de mutação |
| Campanha disparar para contato inativo | Auditoria dos pontos de seleção no plano; teste por superfície; teste de mutação no filtro |
| Migração afetar mais contatos que o esperado | Contagem como pré-condição de rollout, com aprovação humana |
| Operador achar que "Inativar" apaga | Texto do diálogo explicando que o histórico é preservado e o contato volta se o cliente escrever |
| Banner virar ruído | Só na reativação não reconhecida, e dispensável; o histórico permanente fica no painel, não na frente |
