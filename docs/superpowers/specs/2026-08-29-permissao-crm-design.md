# Permissão dedicada para o módulo CRM

- **Data:** 2026-08-29
- **Status:** Aprovada — pronta para plano de implementação
- **Autor/revisão:** Ivan Coelho (product) · design colaborativo
- **Escopo:** primeiro de quatro trabalhos independentes derivados da revisão da Fase 2. Os outros três — tempo real por WebSocket, semântica do `PUT` sem `description`, e tipificação de motivo — têm specs próprias e **não** entram aqui.

## 1. O problema

O módulo de ocorrências inteiro é autorizado por `chat:read` e `chat:write`. Todos os handlers de `occurrences.go`, `occurrence_stages.go` e `occurrence_send.go` chamam `requireAuth` com `models.ResourceChat`.

Duas consequências, ambas em produção hoje:

**Não há como separar CRM de atendimento.** Quem enxerga conversa enxerga ocorrência, e não existe papel que tenha um sem o outro. Isso impede vender ou conceder o módulo separadamente.

**Administrar o funil está sob a permissão de configurações gerais.** Criar, editar e apagar etapas exige `settings.general:write` ([occurrence_stages.go:50](../../../internal/handlers/occurrence_stages.go), [:103](../../../internal/handlers/occurrence_stages.go), [:185](../../../internal/handlers/occurrence_stages.go)) — decisão explícita da Fase 1, que preferiu não criar permissão nova naquele momento. Não há falha de autorização aqui: o agente não tem `settings.general:write` e portanto já não administra etapas, e a tela também já exige a mesma permissão no frontend ([router/index.ts:306](../../../frontend/src/router/index.ts)).

O problema é de **granularidade**: não dá para deixar alguém administrar o funil sem também entregar as configurações gerais da organização. São coisas de escopo bem diferente sob a mesma chave.

> **Correção registrada.** Uma versão anterior desta spec afirmava que as mutações de etapa exigiam apenas `chat:write` e que qualquer agente podia apagar etapas do funil. Era falso — a afirmação foi feita sem ler os três handlers. Isso importa porque muda a natureza da mudança: mexer nas etapas aqui é **refinamento de granularidade**, não correção de falha de segurança, e nenhum agente perde capacidade que hoje exerça.

## 2. Objetivos e não-objetivos

**Objetivos**

- Permissões próprias para o CRM, visíveis e editáveis em Configurações › Funções.
- Configuração de etapas passa a ser ato de gestor.
- Nenhum usuário legítimo perde acesso no dia do deploy.

**Não-objetivos**

- **Mudar visibilidade.** `visibleOccurrences`, `resolveAssignee` e `loadAuthorizedOccurrence` não são tocados. Quem vê o quê continua herdado do escopo de conversa.
- **`view_all` / `view_team` para ocorrências.** As ações existem no modelo, mas dar a um supervisor visão total do CRM sem visão total do chat é mudança de visibilidade, não de permissão. Misturar as duas coisas numa migração de produção é como se perde o rastro do que quebrou.
- **Permissão de exclusão de ocorrência.** Não existe endpoint de exclusão; uma permissão sem endpoint atrás protege nada.
- Tipificação, tempo real e a semântica do `PUT`.

## 3. Os recursos

Dois recursos, com o mínimo de ações que têm endpoint por trás:

| Recurso | Ações | Cobre |
|---|---|---|
| `occurrences` | `read`, `write` | ocorrências, timeline, envio de protocolo **e a leitura das etapas** |
| `occurrences.stages` | `write`, `delete` | criar, editar e apagar etapas |

### Por que `occurrences.stages` não tem `read`

Listar etapas é parte de **usar** o CRM, não de administrá-lo: o quadro não renderiza sem elas, o seletor de etapa do detalhe não existe sem elas, e o painel do chat as consome. Portanto `ListOccurrenceStages` fica sob `occurrences:read`.

Uma permissão de leitura separada não protegeria nada distinto e criaria uma caixa a mais para alguém desmarcar e quebrar o quadro sem entender por quê.

### Mapeamento dos handlers

| Handler | Arquivo | Permissão |
|---|---|---|
| `CreateOccurrence` | `occurrences.go` | `occurrences:write` |
| `ListOccurrences` | `occurrences.go` | `occurrences:read` |
| `ListContactOccurrences` | `occurrences.go` | `occurrences:read` |
| `GetOccurrence` | `occurrences.go` | `occurrences:read` |
| `UpdateOccurrence` | `occurrences.go` | `occurrences:write` |
| `ChangeOccurrenceStage` | `occurrences.go` | `occurrences:write` |
| `ListOccurrenceEvents` | `occurrences.go` | `occurrences:read` |
| `CreateOccurrenceEvent` | `occurrences.go` | `occurrences:write` |
| `SendOccurrenceProtocol` | `occurrence_send.go` | `occurrences:write` |
| `ListOccurrenceStages` | `occurrence_stages.go` | `occurrences:read` |
| `CreateOccurrenceStage` | `occurrence_stages.go` | `occurrences.stages:write` |
| `UpdateOccurrenceStage` | `occurrence_stages.go` | `occurrences.stages:write` |
| `DeleteOccurrenceStage` | `occurrence_stages.go` | `occurrences.stages:delete` |

Só a constante passada a `requireAuth` muda. Nenhuma outra linha desses handlers é tocada.

## 4. O backfill

### A regra

Para cada papel de cada organização:

| Se o papel tem | Ganha |
|---|---|
| `chat:read` | `occurrences:read` |
| `chat:write` | `occurrences:write` |
| `settings.general:write` | `occurrences.stages:write` e `occurrences.stages:delete` |

A regra é **equivalência exata com a capacidade de hoje**: quem administra etapas neste momento é precisamente quem tem `settings.general:write`, então são precisamente esses que recebem as permissões novas. Ninguém ganha nem perde capacidade no deploy.

`roles:write` **não** entra na regra. Administrar papéis e administrar o funil são coisas distintas, e quem tem só o primeiro não configura etapas hoje — incluí-lo concederia acesso novo numa migração cujo propósito declarado é não alterar o acesso de ninguém.

### Idempotência

Sem tabela de controle de migração — o repositório não tem uma, e inventar uma para isto seria desproporcional. A guarda é pela forma do dado, seguindo `BackfillChatbotFlowGraph`, que só toca linhas ainda no formato antigo.

A guarda aqui é **por organização**:

> Se a organização já tem qualquer papel com qualquer permissão cujo recurso comece por `occurrences`, a organização inteira é pulada.

Roda uma vez por organização; repetir é inócuo.

**O caso que ela não cobre**, registrado deliberadamente: se um administrador remover as permissões `occurrences` de **todos** os papéis da organização e o serviço reiniciar, o backfill roda de novo e as devolve. É estreito, visível e reversível na própria tela de funções. A alternativa — um ledger de migrações — introduz infraestrutura nova para cobrir um caso que ninguém relatou.

### O backfill é puramente aditivo

Ele **só concede** permissões; nunca revoga. Isso não é um detalhe de implementação, é a propriedade que torna o rollback seguro: se a versão anterior voltar, ela exige `chat:*`, que todos os papéis continuam tendo intactos. Voltar atrás não deixa ninguém sem acesso.

Nenhuma etapa da implementação pode remover `chat:*` de papel algum.

## 5. Deploy — pré-condição, não recomendação

O bloco de migração roda em [main.go:155](../../../cmd/whatomate/main.go) e o `ListenAndServe` está na [linha 279](../../../cmd/whatomate/main.go). O backfill entra nesse bloco, ao lado de `BackfillChatbotFlowGraph`.

A consequência é que, **se o contêiner subir com `-migrate`, não existe janela de 403**: o backfill termina antes de a aplicação aceitar a primeira requisição. A ordem é garantida por construção, não por disciplina operacional.

O risco inverso é maior que a janela. O bloco inteiro é condicional:

```go
if *migrate {
```

Se produção **não** passa `-migrate`, o backfill nunca roda e o CRM fica inacessível para todos — permanentemente, não por alguns segundos — até alguém executar a migração à mão.

A falha é de **autorização, não de dados**: as ocorrências, os protocolos e as etapas continuam íntegros no banco; o que falta é o vínculo entre papéis e as permissões novas, e todo endpoint do módulo passa a responder 403. Conceder as permissões devolve o acesso, e nada precisa ser recuperado.

### Pré-condição verificável

Antes do rollout, confirmar que o comando da aplicação no `docker-compose.yml` de produção inclui `-migrate`. O script `deploy-whatc` faz `docker compose up -d app` e não passa argumentos próprios, então a flag tem de estar no compose.

- **Se estiver:** nada mais é necessário. A ordem está garantida.
- **Se não estiver:** o rollout ganha uma etapa explícita antes de subir a nova imagem — executar o binário novo com `-migrate` uma vez contra o banco de produção, e só então liberar a aplicação.

Essa verificação é condição de partida do deploy. Sem ela confirmada, o rollout não começa.

### Ordem de rollback

Rollback é seguro em qualquer ponto, pela propriedade aditiva de §4. Não há passo de desfazer.

## 6. Frontend

Quatro pontos, todos pequenos:

| Onde | Mudança |
|---|---|
| `router/index.ts` — rotas `crm/occurrences` e `crm/occurrences/:id` | `permission: 'chat'` → `'occurrences'` |
| `router/index.ts` — rota `settings/occurrence-stages` (nas duas listas onde aparece) | `permission: 'settings.general'` → `'occurrences.stages'` |
| `components/layout/navigation.ts` — entrada `nav.crm` | `permission: 'chat'` → `'occurrences'` |
| `lib/constants.ts` — `RESOURCE_LABELS` | duas entradas: `occurrences` e `occurrences.stages` |

O editor de funções monta os grupos sozinho a partir da API (`permissionGroups`, `stores/roles.ts:47`), com fallback para o nome do recurso capitalizado.

`RESOURCE_LABELS` fica em `lib/constants.ts` e é um mapa de strings **fixas em inglês**, não passa pelo i18n — as entradas existentes são `users: 'Users'`, `contacts: 'Contacts'` e assim por diante. Portanto **esta spec não acrescenta nenhuma chave de tradução**; são duas entradas em inglês seguindo o estilo do mapa. Sem o rótulo, o fallback exibiria `Occurrences` e `Occurrences.stages` — o segundo é feio o bastante para justificar o mapa.

## 7. Verificação

**Go — autorização**

- Cada endpoint da tabela de §3 devolve **403** para um usuário sem a permissão correspondente.
- Um usuário com `occurrences:read` mas sem `occurrences:write` lê a lista e recebe 403 ao criar.
- **O caso central:** um papel com `chat:write` mas **sem** `settings.general:write` usa ocorrências normalmente e recebe **403** ao criar, editar ou apagar etapa. Este é um teste de **caracterização**, não de regressão: já passa hoje, porque as mutações de etapa já exigem `settings.general:write`. Ele existe para travar a separação depois da troca de permissão — é o que um erro de mapeamento silenciaria, ligando administração de funil a `occurrences:write`.
- **A regressão de verdade:** um papel com `settings.general:write` continua administrando etapas depois do backfill. É a capacidade que a migração precisa preservar, e a única cuja perda seria invisível até um gestor reclamar.
- `ListOccurrenceStages` responde 200 para quem tem apenas `occurrences:read` — o quadro não pode depender de permissão administrativa.

**Go — backfill**

- Papel com `chat:read` e `chat:write` recebe `occurrences:read` e `occurrences:write`, e **não** recebe as de etapa.
- Papel com `settings.general:write` recebe as duas de etapa.
- Papel com `roles:write` mas **sem** `settings.general:write` **não** recebe as de etapa — a regra é equivalência, não generosidade.
- Papel sem nenhuma permissão de chat não recebe nada.
- **Nenhuma permissão `chat:*` é removida de papel algum** — asserção explícita, porque é o que garante o rollback.
- Rodar o backfill duas vezes não duplica vínculos.
- Organização que já tem qualquer permissão `occurrences*` é pulada inteira: um papel sem as permissões nessa organização continua sem elas depois de rodar.

**E2E**

- Um papel sem `occurrences` não vê o item de CRM no menu e é barrado ao navegar direto para `/crm/occurrences`.
- Um papel com `occurrences` mas sem `occurrences.stages` usa o quadro e não vê a tela de configuração de etapas.

**Teste de mutação:** inverter o mapeamento de um handler de mutação de etapa para `occurrences:write` deve deixar o teste de regressão de §7 vermelho.

## 8. Riscos

| Risco | Mitigação |
|---|---|
| Produção sem `-migrate`: CRM inacessível para todos, permanentemente | Pré-condição verificável de §5; sem ela confirmada, o rollout não começa |
| Gestor perder a administração do funil no deploy | O backfill mapeia `settings.general:write` um-para-um; teste de regressão dedicado em §7 |
| Backfill re-conceder permissão removida de propósito | Aceito e registrado em §4; estreito e reversível |
| Rollback deixar usuários sem acesso | Impossível por construção: o backfill nunca revoga `chat:*` |
| Erro de mapeamento passar despercebido | O teste de regressão de §7 e o teste de mutação |
