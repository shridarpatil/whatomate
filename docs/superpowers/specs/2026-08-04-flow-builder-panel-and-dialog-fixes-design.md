# Construtor de fluxo e modais — botões mortos, modal que estoura e painel recolhível

- **Data:** 2026-08-04
- **Status:** Aprovada — pronta para plano de implementação
- **Autor/revisão:** Ivan Coelho (product) · design colaborativo
- **Branch:** `feature/flow-builder-panel-and-dialog-fixes`
- **Arquivos-alvo (produção):**
  - `frontend/src/views/chatbot/ChatbotFlowBuilderView.vue`
  - `frontend/src/views/chat/ChatView.vue`
  - `frontend/src/components/ui/dialog/DialogContent.vue`
  - `frontend/src/components/ui/alert-dialog/AlertDialogContent.vue`
  - `frontend/src/i18n/locales/pt-BR.json`, `frontend/src/i18n/locales/en.json`
  - `frontend/e2e/tests/chatbot/flow-builder.spec.ts`, `frontend/e2e/pages/FlowsPage.ts` (onde vive `ChatbotFlowBuilderPage`)

## 1. Contexto

Três problemas reportados a partir do uso em produção, todos no frontend:

1. No construtor de fluxo, o diálogo "Alterações não salvas" abre, mas **os botões "Ficar" e "Sair" não fazem nada**. O usuário fica preso no modal.
2. O modal "Definir responsável" (chat) **não se ajusta ao conteúdo**: com nomes longos, o `Select` e o campo de busca aparecem *fora* do cartão branco e os badges de papel saem cortados.
3. A área de edição do fluxo **fica pequena** em fluxos grandes; o painel de configurações à direita ocupa 420px fixos sem forma de recolher.

O sistema está em produção. Cada mudança abaixo foi escolhida para ter o menor raio de alcance possível.

## 2. Objetivos e não-objetivos

**Objetivos**

- Restaurar o funcionamento de "Ficar"/"Sair" no construtor de fluxo.
- Fazer os modais respeitarem a própria largura e a altura da janela, com rolagem real quando o conteúdo passar do limite.
- Dar ao autor de fluxos controle sobre a largura da área de edição (recolher e redimensionar), com o estado preservado entre sessões.

**Não-objetivos**

- **Não** adicionar o guard de rota (`useUnsavedChangesGuard`) ao construtor de fluxo. Decidido explicitamente: hoje só os botões Voltar/Cancelar disparam o aviso; sair pela sidebar ou fechar a aba não avisa. Fica registrado como dívida (ver §7), fora deste escopo.
- **Não** auditar todos os modais do app atrás do mesmo padrão. A blindagem nos componentes base cobre a classe do problema; a varredura completa fica para outra tarefa.
- **Não** mexer no `useUnsavedChangesGuard`, inclusive no `window.location.href` que ele usa para confirmar a saída (recarrega a página inteira). Fora de escopo.
- **Não** refatorar o `ChatbotFlowBuilderView.vue` (961 linhas) nem o `ChatView.vue`. Mudanças pontuais apenas.

## 3. Problema 1 — Botões "Ficar"/"Sair" mortos

### Causa raiz

`ChatbotFlowBuilderView.vue:959` usa um contrato que não existe:

```vue
<UnsavedChangesDialog v-model:open="cancelDialogOpen" @confirm="confirmCancel" />
```

`UnsavedChangesDialog.vue` declara `props: { open }` e `emits: ['stay', 'leave']`. Portanto:

- `v-model:open` escuta `update:open`, que o componente **nunca emite**;
- `@confirm` escuta `confirm`, que o componente **nunca emite**.

Os dois botões ficam inertes. As outras **11** telas que consomem esse diálogo usam o contrato correto (`:open` + `@stay` + `@leave`); só o construtor de fluxo está fora do padrão.

### Correção

```vue
<UnsavedChangesDialog :open="cancelDialogOpen" @stay="cancelDialogOpen = false" @leave="confirmCancel" />
```

`confirmCancel()` já existe e já faz o correto: fecha o diálogo e chama `router.push('/chatbot/flows')`. Nenhuma outra alteração de lógica.

**Sobre Esc / clique fora:** o `AlertDialog` é controlado por `:open` sem `@update:open`, então Esc não fecha. Esse é o comportamento **das 11 outras telas** — um alert dialog exige uma escolha explícita. Mantemos igual, por consistência.

## 4. Problema 2 — Modais que estouram o próprio cartão

### Causa raiz

`DialogContent` é um **CSS grid** (`grid w-full max-w-lg …`) sem `grid-template-columns` declarado. A coluna implícita usa a função de dimensionamento `auto`, cujo **mínimo** é a maior "contribuição mínima" entre os itens — e essa contribuição **pode ultrapassar a largura do contêiner**, causando overflow.

No diálogo "Definir responsável" cada item da lista é um `Button` (flex) contendo:

- ícone (`shrink-0`),
- `<span class="truncate">` com o nome — e `truncate` aplica `white-space: nowrap`. Como item flex, seu `min-width` é `auto`, logo sua contribuição mínima é **o texto inteiro**, sem encolher;
- `Badge` com o papel (`shrink-0`).

Com "Supervisor Vitória da Conquista" + badge "Supervisor de Unidade", a contribuição mínima do botão chega a ~390px dentro de um cartão de 384px (`max-w-sm`). A trilha do grid cresce para ~390px, **todos** os itens do grid (header, corpo) esticam junto, e o `Select`/`Input` (`w-full`) passam a medir mais que o cartão — exatamente o vazamento visível no print. O `overflow-hidden` do `ScrollArea` corta os badges.

### Bug latente encontrado no mesmo trecho

`ScrollArea` está com `max-h-[280px]` num `ScrollAreaRoot` de altura automática. O `ScrollAreaViewport` do reka-ui aplica `overflow-y: scroll` e a classe `h-full`; como o pai tem `height: auto` (só `max-height`), a porcentagem resolve como `auto` e **o viewport cresce com o conteúdo em vez de rolar**. O `overflow-hidden` do Root simplesmente corta o excesso.

Com os 8 supervisores do caso reportado o conteúdo cabe por pouco dentro dos 280px. A partir de ~10 usuários **os últimos ficam inalcançáveis** — não há rolagem para chegar neles. Corrigido aqui porque é o mesmo sintoma ("o modal não se ajusta ao conteúdo") e a mesma linha de código.

### Correção

**a) Blindagem nos componentes base** — adicionar `grid-cols-[minmax(0,1fr)]` à lista de classes de:

- `components/ui/dialog/DialogContent.vue`
- `components/ui/alert-dialog/AlertDialogContent.vue`

A coluna passa a ter mínimo `0`, então nunca excede a largura do modal. Nenhum modal que já cabe muda de tamanho — só deixa de vazar.

Compatível com quem sobrescreve o display: `cn()` usa `tailwind-merge`, então um consumidor que passa `flex flex-col` (como o preview do fluxo em `ChatbotFlowBuilderView.vue:941` já faz) substitui `grid` e a regra `grid-cols-*` fica inerte.

**b) Diálogo "Definir responsável"** (`ChatView.vue:2846`):

- `DialogContent`: `max-w-sm` → `max-w-md max-h-[85vh] flex flex-col`. O modal cresce com o conteúdo até 85% da altura da janela e nunca ultrapassa a tela.
- `DialogHeader`: `shrink-0`.
- Corpo (`div.py-4.space-y-3`): vira `flex-1 min-h-0 flex flex-col`, para propagar altura definida ao filho.
- `ScrollArea`: `max-h-[280px]` → `flex-1 min-h-0`. Com altura definida vinda do flex, o `h-full` do viewport resolve e a rolagem passa a funcionar de fato.
- `<span class="truncate">` → `class="truncate min-w-0 flex-1 text-left"`. É o `min-w-0` que permite ao item flex encolher; sem ele o `truncate` nunca trunca, só empurra.

**c) Diálogo irmão de transferência** (`ChatView.vue:2911`): mesmo padrão, mesmas classes. Tem a mesma estrutura e o mesmo defeito.

## 5. Problema 3 — Painel direito recolhível e redimensionável

Alvo: `ChatbotFlowBuilderView.vue:785`, hoje `<Card class="w-[420px] … shrink-0 flex flex-col">`.

### Comportamento

- **Recolher/expandir** por botão com chevron.
- **Redimensionar** arrastando a borda esquerda do painel.
- Quando recolhido, permanece uma **faixa fixa de ~28px** na borda direita, com o chevron para reabrir. Sempre visível, impossível de perder.

### Estado

| Item | Valor |
|---|---|
| `panelCollapsed` | booleano, padrão `false` |
| `panelWidth` | número, padrão `420`, limitado a `[320, 720]` |

Persistidos em `localStorage` sob a chave `flow-builder-panel` (segue a convenção kebab-case já usada por `color-mode` em `composables/useColorMode.ts`), como um JSON `{ collapsed, width }`. Leitura defensiva: chave ausente, JSON inválido, valor não numérico ou fora da faixa cai no padrão — nunca quebra a tela.

### Alça de arraste

- Área de acerto de ~5px na borda esquerda do Card, `cursor-col-resize`, com realce visual no hover.
- `pointerdown` + `setPointerCapture` (funciona com mouse e caneta, e não perde o arraste se o ponteiro sair da janela); `pointermove` calcula a nova largura a partir de `clientX`, com clamp; `pointerup`/`pointercancel` encerram e persistem.
- Seleção de texto desabilitada durante o arraste.
- Acessível: `role="separator"` com `aria-orientation="vertical"`, `tabindex="0"`, e setas ←/→ ajustando 20px por toque.

### Proteção contra estado confuso

Ao selecionar um nó com o painel recolhido, **o painel abre sozinho**. Sem isso, clicar num nó não teria efeito visível e pareceria um bug.

### Canvas

O canvas é `flex-1` e reflui sozinho; o VueFlow já observa o próprio tamanho. **Não** chamamos `fitView` no toggle nem no arraste — isso reposicionaria a viewport e tiraria o autor do ponto onde ele estava editando.

### i18n

Três chaves novas sob `flowBuilder`, adicionadas em `pt-BR.json` **e** `en.json`:

| Chave | pt-BR | en |
|---|---|---|
| `flowBuilder.collapsePanel` | Recolher painel | Collapse panel |
| `flowBuilder.expandPanel` | Expandir painel | Expand panel |
| `flowBuilder.resizePanel` | Redimensionar painel | Resize panel |

Usadas como `aria-label`/`title` do botão de toggle e do separador. O repo tem `npm run i18n:keys` para conferir paridade entre os locales.

## 6. Verificação

- `npm run typecheck` e `npm run lint` limpos.
- `npm run i18n:keys` sem chaves faltando.
- **E2E** (`e2e/tests/chatbot/flow-builder.spec.ts`, que já existe e usa o page object `ChatbotFlowBuilderPage`):
  - alterar algo, clicar em Voltar, clicar em **Ficar** → continua no builder e a alteração é preservada;
  - alterar algo, clicar em Voltar, clicar em **Sair** → navega para a lista de fluxos;
  - recolher o painel → a faixa de reabrir fica visível e o conteúdo do painel some;
  - expandir de volta → o conteúdo do painel reaparece.
- **Visual** (correção de CSS, precisa de olho): rodar o app e abrir "Definir responsável" com nomes longos, conferindo que nada vaza do cartão, que os badges aparecem inteiros e que a lista rola quando há muitos usuários.

## 7. Dívida registrada (fora deste escopo)

- O construtor de fluxo não usa `useUnsavedChangesGuard`; sair pela sidebar, por link ou fechando a aba não avisa sobre alterações não salvas, ao contrário das outras 11 telas.
- `useUnsavedChangesGuard.confirmLeave()` navega com `window.location.href`, forçando reload completo da aplicação em vez de usar o router.
- Não foi feita varredura dos demais modais atrás do padrão `truncate` sem `min-w-0`.
