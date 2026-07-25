# Visibilidade por time — permissão `conversations:view_team`

- **Data:** 2026-07-24
- **Status:** Aprovada — pronta para plano de implementação
- **Autor/revisão:** Ivan Coelho (product) · design colaborativo
- **Arquivos-alvo (produção):** `internal/handlers/conversation_visibility.go`, `internal/models/roles.go` (catálogo de permissões), testes em `internal/handlers/conversation_visibility_test.go`

## 1. Contexto e problema

O sistema é multiempresa e multi-camadas de permissão. A operação-alvo é uma **rede de lojas** modelada como **uma única organização** com **times**:

- Um time central **`SAC`**.
- Por loja *N*: times setoriais **`Loja{N}-ADM`**, **`Loja{N}-Logística`** (e outros setores, se houver).

Papéis desejados:

- **Supervisor geral do SAC** — vê todas as conversas (SAC + todas as lojas). Já atendido hoje por `conversations:view_all`.
- **Agentes de setor** — atendem os fluxos/nós do seu setor. Já atendido: os fluxos roteiam por time (`contact.team_id`).
- **Supervisor de loja** — precisa **ver e agir** em **todas as conversas dos times da sua loja** (ADM + Logística), **inclusive as que os agentes já assumiram**, e **nada** de outras lojas.

### O que falta hoje

No modo de **visibilidade estrita** (`strict_conversation_visibility`), a autorização final tem dois pontos em que a resposta é *"o dono é você"*:

- **Ramo A** — transferência ativa para um agente: visível se `transfer.agent_id == usuário`.
- **Ramo D** — sem transferência, conversa em carteira: visível se `assigned_user_id == usuário`.

Os demais ramos (**B** fila do time, **C**/**F** time padrão da conta, **E** time do fluxo) já são por time, e um membro do time os enxerga.

Consequência: **assim que uma conversa é assumida por um agente específico** (pickup, "manter o mesmo agente", ou transferência para agente), **a carteira vence o time** — e só aquele agente (ou quem tem `view_all` global) passa a vê-la. O supervisor de loja, mesmo membro do time, **deixa de ver** exatamente quando mais precisa acompanhar.

Não existe hoje um "ver tudo por time": `conversations:view_all` é global (todas as lojas). Logo:

- `view_all` no supervisor de loja → ele veria **todas** as lojas (indesejado).
- sem `view_all` → ele só vê a fila do time, não o que os agentes assumiram.

## 2. Objetivos e não-objetivos

**Objetivos**

- Um supervisor consegue **ver e agir** em todas as conversas dos **times de que é membro**, inclusive as pertencentes (carteira/transferência) a outros agentes desses times.
- Escala para muitas lojas **sem explosão de papéis** e para usuários em **múltiplos times**.
- **Compatibilidade total com produção**: nenhum usuário existente muda de comportamento.

**Não-objetivos**

- Não alterar a semântica de `conversations:view_all`.
- Não introduzir hierarquia de times ou grupos de supervisão agora (o desenho deixa isso trivial de adicionar depois — ver §9).
- Não alterar como fluxos roteiam (já setam `contact.team_id`).

## 3. Semântica da permissão (inequívoca)

> **`conversations:view_team`** amplia **exclusivamente** o escopo de visibilidade do usuário para incluir **todas as conversas pertencentes aos times dos quais ele é membro**. A permissão **não** concede acesso a conversas fora desses times e **não** altera qualquer outra regra de autorização existente.

Corolários:

- `view_team` **não** é um atalho para `view_all`. Não dá acesso a dados globais.
- O escopo é a **união** dos times do usuário. Um usuário em vários times vê a união — sem código adicional.
- A permissão concede **ver e agir** (`canView` e `canInteract`), simetricamente ao que `view_all` faz globalmente, porém limitada aos times do usuário.

## 4. Requisito não-funcional — ordem de avaliação (mudança aditiva)

> A introdução de `view_team` **não altera a precedência nem a ordem lógica** das regras atuais de autorização. Os ramos existentes (A–F) permanecem **semanticamente idênticos**. Apenas **dois ramos** ganham um análogo condicionado à presença da nova permissão.

Para um usuário **sem** `view_team`, a consulta e a função executam **exatamente** o caminho de hoje — sem ramos adicionais emitidos. A mudança é estritamente aditiva, não uma reescrita do algoritmo.

**Precedência de `view_all` (inalterada).** A presença de `conversations:view_all` continua **encerrando** a avaliação com acesso global, **antes** de qualquer consideração de `view_team`. Ou seja: quem tem `view_all` não passa pela lógica de time — não há verificação redundante a implementar. `view_team` só é consultado para quem **não** tem `view_all`.

## 5. Fonte de verdade e primitiva reutilizável

**Definição formal (associação ativa).** Dois usuários estão no mesmo escopo de time quando existe pelo menos um `team_id` para o qual **ambos** possuem uma **associação ativa** em `team_members`. "Associação ativa" = uma linha de `team_members` **não removida** (o `team_members` usa soft-delete via `BaseModel`; o escopo padrão do GORM já exclui as removidas). Se no futuro surgirem membro inativo, convite pendente ou outro estado, **somente associações ativas** contam para autorização.

A relação é definida **em um único lugar de cada lado**, para que a evolução da modelagem de times altere **um** ponto:

- **Go:** `func (a *App) canViewTeamMember(viewerID, ownerID uuid.UUID) bool` — verdadeiro se `viewerID` e `ownerID` têm associação ativa a um time em comum. (Inclui o caso `viewerID == ownerID`.)
- **SQL:** uma subquery reutilizável **`viewTeamScope(userID)`** = o conjunto de `user_id` (donos) visíveis a `userID` pelo escopo de time — isto é, os que compartilham um time ativo com ele. Forma ilustrativa (o implementador pode usar `JOIN`/`EXISTS`/`CTE` se for melhor — ver §6.2):

  ```sql
  SELECT tm.user_id
  FROM team_members tm
  WHERE tm.team_id IN (
      SELECT team_id FROM team_members WHERE user_id = :userID
  )
  ```

**Invariante obrigatória (Go ↔ SQL):** `viewTeamScope(userID)` representa **exatamente** o conjunto de `ownerID` para os quais `canViewTeamMember(userID, ownerID)` retornaria verdadeiro. Alterar uma implementação **obriga** alterar a outra; o teste-oráculo (§8.1) as mantém em acordo. Nenhuma lógica de "escopo de time" é repetida fora dessas duas definições.

**Nota de nomenclatura.** O nome `canViewTeamMember` expressa a *intenção de autorização* ("o viewer pode ver conversas deste dono pelo escopo de time"), não o mecanismo atual ("compartilha um time"). Assim, se a regra evoluir para hierarquia/regiões/grupos de supervisão, o nome permanece válido e a mudança fica contida nessa única primitiva. (O modelo já tem `TeamMember.Role` — manager/agent por time — que poderia servir de base a uma futura noção de liderança, mas **não** é usado aqui: nesta feature o escopo vem da associação ao time, não do papel dentro dele.)

## 6. A mudança na regra

### 6.1 Função — `authorizeConversation` (fonte única)

Dentro do bloco de modo estrito, após a verificação de `view_all` (inalterada), calcula-se uma vez:

```go
hasViewTeam := a.HasPermission(userID, models.ResourceConversations, models.ActionViewTeam, orgID)
```

E **somente os ramos A e D** ganham uma condição `OU`, atrás de `hasViewTeam`:

- **A** (transferência ativa para agente):
  `*transfer.AgentID == userID` **OU** `(hasViewTeam && a.canViewTeamMember(userID, *transfer.AgentID))`
- **D** (carteira):
  `*contact.AssignedUserID == userID` **OU** `(hasViewTeam && a.canViewTeamMember(userID, *contact.AssignedUserID))`

Os ramos B, C, E, F ficam **idênticos** — o supervisor já os enxerga por ser membro dos times. `canView` e `canInteract` continuam iguais entre si (ver **e** agir). `hasViewTeam` só é avaliado no caminho sem `view_all` (§4).

### 6.2 SQL — `scopeVisibleConversations` (gêmeo)

Os seis ramos atuais (A–F) são emitidos **sem alteração**. Quando `hasViewTeam` é verdadeiro, o escopo é ampliado com os **equivalentes por time dos ramos A e D**, expressos via `viewTeamScope(userID)`:

- **G** (equivalente de A): contatos com **transferência ativa** cujo `agent_id` está em `viewTeamScope(userID)`.
- **H** (equivalente de D): contatos **sem transferência ativa** cujo `assigned_user_id` está em `viewTeamScope(userID)`.

**Requisito (não prescritivo quanto à forma):** o escopo SQL deve incluir G e H usando `viewTeamScope(userID)`; a montagem (`IN (subquery)`, `EXISTS`, `JOIN` ou `CTE`) fica a critério do implementador, desde que preserve a invariante da §5 e o resultado do oráculo (§8.1).

Para usuários sem `view_team`, **G e H não são emitidos** → consulta byte-a-byte igual à de hoje.

> **Nota de equivalência:** G e H são a forma "consulta de conjunto" do mesmo `OU` que a função adiciona aos ramos A e D. A diferença é só de estilo (a função é imperativa, por contato; o SQL é declarativo, por conjunto). A igualdade é garantida pelo teste-oráculo (§8.1) — nenhuma das formas amplia o escopo além dos times do usuário.

### 6.3 Casos-limite (comportamento especificado)

- **Dono sem time.** Se o dono de uma conversa (`assigned_user_id` ou `agent_transfer.agent_id`) **não pertence a nenhum time**, ele não está em `viewTeamScope` de ninguém → a conversa **não** fica visível a nenhum supervisor por `view_team` (só ao próprio dono e a quem tem `view_all`). Ex.: `assigned_user_id = João`, João sem time → o supervisor **não** vê.
- **Supervisor sem time.** Um usuário **com** `view_team` mas **sem nenhum time** tem `viewTeamScope` vazio → **não ganha acesso algum** além do padrão; enxerga **exatamente** o comportamento antigo.
- **Escopo segue o dono (regra esperada).** `view_team` escopa pelo **dono atual** da conversa (agente atribuído ou agente da transferência ativa), não pelo `contact.team_id` original. Como assumir/atribuir passa por `canInteractWithConversation` (mesma regra de `canViewConversation`), **um agente só assume conversa que já enxerga** — ou seja, do seu próprio time; cross-team pickup por agente **não ocorre** no modo estrito. A exceção é deliberada: um usuário com `view_all` (admin/manager) pode atribuir uma conversa a um agente de outro time — e então a visibilidade **segue o novo dono** (o supervisor do time desse agente passa a vê-la). Este é o comportamento **desejado**: quem detém a conversa define o escopo de supervisão.

### 6.4 Espelhamento obrigatório

Função e SQL devem devolver **exatamente** o mesmo conjunto. Isso é garantido pelo teste-oráculo existente (§8.1), estendido para cobrir `view_team`.

## 7. Modelagem operacional da rede (mapa de times e papéis)

**Princípio:** o **papel** não muda por loja; o que localiza alguém numa loja é a **participação em time**. Assim, N lojas usam **6 papéis fixos** e times por loja. Nunca "um papel por loja".

**Um usuário pode pertencer a vários times.** O escopo de `view_team` é a **união** dos seus times. Isso cobre, só com configuração de participação:

- supervisor de duas (ou mais) lojas;
- gerente regional (membro de todos os times da região);
- cobertura de férias (adicionar temporariamente ao time);
- agente compartilhado entre unidades.

**Redação canônica:** *"o supervisor é membro de todos os times que supervisiona"* (sem acoplar a um número fixo de setores).

### Papéis (6 fixos, reutilizados em todas as lojas)

| Papel | Permissões-chave | Membro de… | Enxerga |
|---|---|---|---|
| **admin** | todas (config + regras) | — | tudo · única autoridade de configuração |
| **SAC-Supervisor** | `conversations:view_all` + chat + chat.assign + analytics.agents | `SAC` | tudo (SAC + todas as lojas). *Admin concede/retira o `view_all` = "habilitar ou não".* |
| **SAC-Agente** | chat + contacts:read + tags:read + transfers | `SAC` | conversas do SAC no seu escopo |
| **Loja-Supervisor** *(novo)* | `conversations:view_team` + chat + chat.assign + analytics.agents + transfers | todos os times que supervisiona | tudo dos seus times (união) · nada fora deles |
| **Loja-Agente-Logística** | chat + contacts (ver/criar) + tags + transfers | `Loja{N}-Logística` | conversas de logística no seu escopo |
| **Loja-Agente-ADM** | chat + contacts (ver/criar) + tags + transfers + enviar modelo | `Loja{N}-ADM` | conversas de adm no seu escopo |

### Contas WhatsApp / roteamento

- Cada loja usa seu(s) número(s). A **equipe padrão** da conta deve ser um time do qual o supervisor participa (um time de "recepção" da loja ou um dos setoriais), para que conversas **ainda não roteadas** já sejam vistas pela loja (ramo F) e não caiam no limbo "só `view_all`".
- O **fluxo** roteia por setor com nó de botão/transferência setando `team_id = Loja{N}-Logística`/`Loja{N}-ADM` — comportamento já existente.
- O **SAC** recebe pelo seu número/fluxo (equipe padrão = `SAC`); o SAC-Supervisor vê tudo por `view_all`.

### Nota operacional

`view_team` (visibilidade) é **independente** de receber fila (distribuição automática). Se o supervisor não deve entrar no rodízio, basta marcá-lo indisponível para atribuição — configuração, não código.

## 8. Plano de testes

### 8.1 Teste-oráculo estendido

`TestVisibilityScopeMatchesFunction` ganha fixtures com um usuário `view_team` (supervisor) e prova, para **cada** cenário, que `scopeVisibleConversations` (SQL) devolve **exatamente** os contatos que `canViewConversation` (função) permite:

- conversa na carteira de um colega de time (deve ver);
- conversa transferida ativamente a um colega de time (deve ver);
- conversa na fila do time (deve ver);
- conversa de agente de **outro** time / outra loja (**não** deve ver);
- a própria conversa do supervisor (deve ver).

### 8.2 Testes de comportamento

Espelhando `conversation_visibility_test.go`:

1. Supervisor `view_team` vê e **interage** com conversa na carteira de colega de time.
2. Supervisor `view_team` vê conversa transferida ativamente a colega de time.
3. Supervisor `view_team` **não** vê conversa de agente de outro time (outra loja).
4. **Multi-time:** supervisor de duas lojas vê as duas; não vê uma terceira.
5. `view_team` **não** concede visão global: conversa sem relação de time (nenhum time do supervisor) → não vê.
6. **Regressão:** agente **sem** `view_team` vê exatamente o de hoje — asserção explícita de que nada mudou (mesmos resultados dos testes atuais).
7. **Supervisor sem time:** usuário com `view_team` e **zero times** não ganha acesso adicional (comportamento antigo).
8. **Dono sem time:** conversa na carteira de um agente sem nenhum time → supervisor `view_team` **não** vê.
9. **`view_team` + `view_all`:** usuário com ambos vê tudo (globalmente), provando que `view_all` continua vencendo mesmo quando `view_team` está presente.

### 8.3 Suíte e validação

- `go test ./...` verde (cientes das duas falhas pré-existentes de ambiente: symlink no Windows e um teste de timestamp intermitente — confirmadas alheias a esta mudança).
- **Validação ao vivo** na stack: criar papel Loja-Supervisor + times + agentes; reproduzir supervisor vendo o que os agentes da loja assumiram e **não** vendo a outra loja.

## 9. Compatibilidade, rollout e segurança

- **Catálogo:** adicionar `conversations:view_team` a `DefaultPermissions()` (aparece no editor de Funções). Descrição: *"Ver e agir em todas as conversas dos times de que o usuário é membro."* Constante `ActionViewTeam = "view_team"`.
- **Padrões de papel:** **não** adicionar a nenhum papel de sistema em `SystemRolePermissions()`. (O `admin` recebe todas as permissões automaticamente, mas já tem `view_all` → o ramo novo é inócuo para ele. `manager`/`agent` **não** recebem.) A feature fica **inerte** até um admin criar o papel Loja-Supervisor e conceder a permissão.
- **Garantia operacional:** a simples **existência** da nova permissão no catálogo **não altera qualquer decisão de autorização** enquanto ela não for **atribuída a um usuário**. Nenhum comportamento muda no deploy; muda apenas quando um admin concede `view_team` a alguém.
- **Seeding:** garantir que a nova permissão seja semeada nas organizações existentes pelo mesmo mecanismo que já semeia `DefaultPermissions`, para aparecer no editor sem intervenção manual.
- **Auditoria de `view_all`:** mapear **todos** os usos de `conversations:view_all` (listagem, leitura, ações, exportação) e decidir, caso a caso, se `view_team` precisa de tratamento paralelo. A maioria passa por `scopeVisibleConversations` (coberto); os demais são confirmados individualmente para não deixar brecha nem vazamento.
- **Checklist de segurança:** grep confirmando que nenhum papel de sistema recebe `view_team` por padrão · diff confirmando que os seis ramos atuais ficaram intactos · teste-oráculo verde · testes de regressão verdes.

### Evolução futura (fora de escopo agora)

Como o escopo está numa única primitiva (`canViewTeamMember` em Go) e numa única subquery (`viewTeamScope` em SQL), evoluir para **times hierárquicos**, **regiões** ou **grupos de supervisão** significa alterar **apenas** essas duas definições — sem tocar nos ramos de autorização.

**Tipos de time (aviso para a evolução).** Caso o sistema passe a distinguir tipos de time (organizacional, operacional, temporário — ex.: um time efêmero de campanha), `canViewTeamMember`/`viewTeamScope` deverão considerar **apenas os tipos destinados à autorização de visibilidade**. Isso evita que um time temporário amplie inadvertidamente a visibilidade de um supervisor. Enquanto houver um único tipo de time, essa distinção não é necessária.

## 10. Resumo das garantias

- ✅ Compatibilidade total com produção (caminho inalterado sem a permissão).
- ✅ Mudança estritamente aditiva; precedência e ramos A–F preservados.
- ✅ Sem alteração de comportamento para usuários existentes.
- ✅ Escala para múltiplas lojas e para usuários em múltiplos times.
- ✅ Uma única fonte de verdade (`authorizeConversation`) espelhada em SQL.
- ✅ Espelhamento obrigatório Go↔SQL coberto pelo teste-oráculo.
- ✅ Rollout controlado via nova permissão (inerte por padrão).
- ✅ Sem alteração da semântica de `view_all`.
