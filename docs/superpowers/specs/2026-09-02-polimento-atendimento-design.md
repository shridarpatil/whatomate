# Polimento de atendimento — tooltips, copiar, renomear contato e rótulos no card

- **Data:** 2026-09-02
- **Status:** Aprovada — pronta para plano de implementação
- **Autor/revisão:** Ivan Coelho (product) · design colaborativo
- **Escopo:** quatro melhorias pequenas e independentes, agrupadas por serem todas de interface e de baixo risco. Uma delas — renomear o contato — traz uma permissão nova e por isso puxa backend.

## 1. Os quatro itens

| # | Item | Toca backend? |
|---|---|---|
| 1 | Tooltips nos ícones do menu lateral quando recolhido | não |
| 2 | Botão de copiar o nome e o telefone do contato | não |
| 3 | Agente renomeia o contato pela tela de informações | **sim** |
| 4 | Rótulos "Contato" e "Responsável" no card do Kanban | não |

Os itens 1, 2 e 4 são autocontidos. O item 3 é o que carrega a decisão de arquitetura desta spec.

## 2. Tooltips no menu recolhido

`AppLayout.vue` já tem `isCollapsed` e encolhe a barra para `w-16`, mas não importa `Tooltip` nenhum — recolhido, os ícones ficam sem identificação.

Cada item de navegação ganha um tooltip **exibido apenas quando `isCollapsed` é verdadeiro**, com o mesmo rótulo i18n que o item já mostra expandido. Nenhuma string nova: reaproveita `item.name`.

Escopo nos itens de navegação. O `OrganizationSwitcher` já recebe `:collapsed` e resolve o próprio caso; fica fora.

## 3. Copiar nome e telefone

`ContactInfoPanel.vue:274` e `:278` mostram nome e telefone como texto puro.

Cada um ganha um botão de copiar seguindo o padrão que a Fase 1 já estabeleceu para o protocolo (`OccurrenceDetailView.vue:214-218`): o componente `IconButton` com `:icon="Copy"` e `:label`, e `toast.success(t('common.copiedToClipboard'))` como retorno.

`common.copiedToClipboard` **já existe** e é reaproveitada. As duas chaves novas são só os rótulos acessíveis dos botões, nos dois locales.

O que é copiado é o dado, não o que está na tela: o botão do nome copia `contact.name` quando há nome, e o telefone quando não há — que é exatamente o que o `h4` já exibe. O botão do telefone copia sempre `contact.phone_number`.

## 4. Renomear o contato — a parte que decide

### O estado atual

O papel `agent` tem `contacts:read` e nada mais, com o comentário `// Contacts (read only)` ([roles.go:333](../../../internal/models/roles.go)). O `UpdateContact` exige `contacts:write`, e esse endpoint altera o contato **inteiro**.

Dar `contacts:write` ao agente resolveria o pedido e entregaria junto tudo o que o `UpdateContact` aceita hoje — e, pior, tudo o que ele vier a aceitar amanhã, sem ninguém reparar.

### A decisão

Recurso próprio, **`contacts.name`**, com a ação `write`, atrás de **endpoint próprio**:

```
PUT /api/contacts/{id}/name
```

Isso não é invenção: é o padrão que o arquivo já usa para operações estreitas — `/assign`, `/tags`, `/status` são todas rotas próprias com handler próprio ([main.go:644-646](../../../cmd/whatomate/main.go)). Um endpoint, um portão. Um caminho condicional dentro do `UpdateContact` teria de acertar quais campos ignorar, e erra silenciosamente no dia em que alguém acrescentar um campo.

O handler segue `UpdateContactTags` como molde, trocando a permissão exigida.

### A visibilidade continua valendo

O agente só renomeia contato com o qual **pode interagir**. O handler chama `canInteractWithConversation`, como os outros handlers de contato fazem.

Sem isso, a permissão nova viraria um furo lateral: um agente renomearia contatos que sequer enxerga na lista, contornando o escopo de conversa que a Fase 1 construiu e testou.

### O backfill é ampliação deliberada, não equivalência

Este é o ponto em que esta spec **difere conscientemente** da spec da permissão do CRM, e o contraste precisa estar escrito para ninguém achar que a regra mudou de dono no meio do caminho.

Lá a regra era equivalência exata: ninguém ganha capacidade nova. Aqui o pedido do produto é justamente **dar ao agente algo que ele não tem hoje**, em nome da agilidade no atendimento.

| Se o papel tem | Ganha | Por quê |
|---|---|---|
| `contacts:write` | `contacts.name:write` | equivalência — já podia renomear |
| `chat:write` | `contacts.name:write` | **ampliação deliberada** — é quem atende |

A segunda linha concede acesso novo. É intencional, é o pedido, e fica registrada como tal.

O backfill segue a mesma disciplina do backfill do CRM: **puramente aditivo**, nunca revoga, e idempotente por guarda de organização — se a organização já tem qualquer papel com `contacts.name:write`, é pulada inteira.

`agent` passa a ter `contacts.name:write` em `SystemRolePermissions()`, para organizações novas.

### A interface

Em `ContactInfoPanel.vue`, o nome vira editável no lugar — clique, campo, salvar — visível apenas para quem tem `contacts.name:write`. Quem não tem continua vendo o nome como hoje.

Salvar recarrega o contato para que o cabeçalho do chat e a lista reflitam o nome novo sem recarregar a página.

## 5. Rótulos no card do Kanban

`OccurrenceCard.vue` mostra contato e responsável em duas linhas seguidas, sem distinção — é impossível saber qual é o cliente e qual é o atendente.

Cada linha ganha seu rótulo, `Contato:` e `Responsável:`, em tom mais fraco que o valor. Duas chaves novas nos dois locales.

O cartão continua sem cor de etapa: a coluna já carrega a cor, e todos os cartões dela estão na mesma etapa.

## 6. Verificação

**Go**

- `PUT /api/contacts/{id}/name` devolve **403** sem `contacts.name:write`.
- Devolve **403** para contato fora do escopo de conversa do usuário, **mesmo com** a permissão — é a garantia de §4.
- Renomeia com sucesso para quem tem a permissão e enxerga o contato.
- Nenhum outro campo do contato muda: telefone, tags e atributos ficam intactos depois da chamada.
- Backfill: papel com `contacts:write` recebe; papel com `chat:write` recebe; papel sem nenhum dos dois **não** recebe; nenhuma permissão existente é removida; rodar duas vezes não duplica; organização já migrada é pulada.

**Playwright**

- Com o menu recolhido, passar sobre um ícone mostra o nome do menu.
- Copiar o telefone põe o número na área de transferência.
- Um agente renomeia o contato e o nome novo aparece no cabeçalho do chat.
- Um papel sem `contacts.name:write` não vê o controle de edição.
- O card do Kanban mostra os dois rótulos.

**Teste de mutação:** remover a checagem de visibilidade do handler de renomear deve deixar vermelho o teste de contato fora de escopo.

## 7. Riscos

| Risco | Mitigação |
|---|---|
| Permissão nova não chega aos papéis existentes | Backfill aditivo, no bloco de migração antes do `ListenAndServe`, como o do CRM |
| Renomear virar furo na visibilidade | `canInteractWithConversation` no handler, com teste de mutação dedicado |
| Endpoint estreito virar largo com o tempo | Rota própria com um só campo no corpo; qualquer campo novo exige decisão explícita |
| Tooltip aparecer com o menu expandido | Condicionado a `isCollapsed`, coberto por teste |
