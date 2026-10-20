# Adaptação do PlusZap para SaaS no Whatomate

## Repositórios analisados

- Destino: `D:/whatomate` (Go, GORM/PostgreSQL, Vue 3).
- Referência: `pluszap-web/` dentro desse repositório (TypeScript/Sequelize, React).
- O caminho antigo `D:/Projetos_Javascript/whatomate` não contém mais o código.

## Comparação funcional

| Área | PlusZap | Whatomate | Adaptação |
| --- | --- | --- | --- |
| Empresas | Company, companyId | Organization, organization_id, UserOrganization | Reutilizar organizações e associação de usuários; não duplicar tenants |
| Administração | Empresas e planos | Superadministrador e permissões granulares | Painel comercial exclusivo do superadministrador |
| Planos | Preço, recorrência, trial, usuários, conexões, filas e módulos | Catálogo comercial e painel administrativo | Valores em centavos, limites informativos e período de teste |
| Assinaturas | Subscriptions e dueDate em Company | Uma assinatura por organização | Vínculo manual, status, vencimento e renovação após pagamento Pix |
| Cobrança | Invoices; Asaas, Mercado Pago, Stripe e Efí | Faturas Pix e conciliação | Mercado Pago principal e Efí de contingência; sem Asaas ou Stripe |
| Pagamento confirmado | Webhooks estendem vencimento | Conciliação autenticada por API | Confere referência, valor e moeda; renova uma vez em transação; funciona em localhost |
| Atendimento | Tickets, filas, contatos e transferências | Conversas, equipes, contatos e transferências | Mapear conceitos, preservando histórico |
| Automação | Fluxos, palavras-chave e IA | Fluxos, palavras-chave, IA e contexto | Reutilizar implementação existente e acrescentar direitos comerciais depois |
| Campanhas | Campanhas e agendamentos | Campanhas e templates Meta | Comparar agendamentos e relatórios antes de portar |
| Telefonia | Rotas de chamadas WABA | Chamadas, URA, transferências e gravações S3 | Preservar integração existente |
| Canais | Baileys, WABA e opções Facebook/Instagram | WhatsApp Cloud API | Adicionar adaptadores separados; Baileys não é substituição direta do cliente Meta |
| CRM | Kanban, tarefas, carteira, agenda | Contatos, tags e ações personalizadas | Módulos adicionais, não cobertos pela gestão comercial |
| Recursos próprios | Consignado, Corban, bases, links curtos | Sem equivalentes diretos | Definir requisitos de negócio e fontes de dados antes de portar |
| Personalização | Layout, configurações, anúncios e ajuda | Configurações e traduções | Comparar white-label, onboarding e comunicação entre tenants |

Fontes principais: `pluszap-web/backend/src/models/{Company,Plan,Subscriptions,Invoices}.ts`,
`controllers/SubscriptionController.ts`, `services/CompanyService/CreateCompanyService.ts`,
`frontend/src/pages/`; `internal/models/models.go`, `handlers/organization.go`,
`handlers/auth.go` e `frontend/src/components/layout/navigation.ts`.

## Ordem de implementação

1. Catálogo comercial e associação de assinatura às organizações, com visualização de uso.
2. Gateway escolhido, faturas, checkout, webhooks autenticados e conciliação.
3. Cadastro de empresas com teste, renovação, cancelamento e recuperação de inadimplência.
4. Aplicar limites atomicamente nos pontos de criação e execução, inclusive API, workers e webhooks.
5. Migrar os módulos restantes por equivalência funcional (CRM, canais, agenda e personalização).

## Situação atual

- Implementado e aplicado no Docker local: catálogo, vínculos comerciais, faturas
  Pix, Mercado Pago principal, Efí por contingência explícita, cancelamento,
  reemissão e conciliação periódica.
- Verificado com provedores simulados e PostgreSQL descartável: isolamento,
  permissões, recuperação de falhas, renovação única e divergência de valor.
- Build Docker e verificação de tipos Vue concluídos. Tabelas `saas_plans`,
  `saas_subscriptions` e `saas_invoices` presentes no banco local.
- Emissão desativada por padrão. Mercado Pago configurado localmente pelo
  administrador, com emissão de Pix verificada após corrigir o formato de validade.
  Pagamento real e renovação após consulta foram confirmados pelo administrador.
  Webhook autenticado e atualização automática da tela foram acrescentados;
  publicação e configuração do webhook no domínio público ainda são necessárias.
  Efí fica para uma etapa futura.
- Cadastro de empresas com trial pelo superadministrador implementado:
  [guia de cadastro](saas-onboarding.md). Inclui responsável, perfis, assinatura
  e auditoria em transação; não há inscrição pública de empresas nesta etapa.
- Próximas entregas: regras comerciais de atraso
  e cancelamento, aplicação atômica dos limites e módulos adicionais do PlusZap.
  Cartão e débito recorrente ainda não estão implementados.

## Limites da primeira etapa

**Atualização da etapa de pagamentos:** renovação por Pix, cancelamento,
reemissão e conciliação foram acrescentados. Consulte [saas-payments.md](saas-payments.md)
para configuração e limites. A descrição abaixo documenta a base comercial;
o restante da migração SaaS continua pendente.

Implementado: **Configurações → Planos e assinatura** (`/settings/billing`).
Superadministradores criam e editam planos, desativam planos e vinculam uma
organização a um plano ativo. O vínculo permite definir situação e vencimento.
Administradores com `settings.general:read` consultam o plano da organização e
uso de usuários, conexões e equipes. As alterações foram aplicadas no Docker local.

Tabelas novas: `saas_plans` e `saas_subscriptions`. Nenhuma organização existente
foi vinculada automaticamente. `-1` significa limite ilimitado; `0`, nenhuma cota.
Preço em centavos: `9900` representa R$ 99,00. Não há exclusão de planos;
desative-os para impedir novos vínculos e preservar os existentes.

O catálogo e a associação comercial não cobram dinheiro nem suspendem operações.
As organizações existentes continuam funcionando sem assinatura vinculada.
Os limites cadastrados são exibidos para acompanhamento, mas ainda não bloqueiam recursos.
Uma assinatura administrada manualmente não é prova de pagamento e não substitui faturas.
Somente superadministradores alteram planos e vínculos; administradores autorizados
da organização consultam seu próprio plano e uso.

Antes de ativar débito recorrente, definir dados fiscais obrigatórios,
recorrência, tolerância de atraso, política de cancelamento e comportamento de mudança de plano.
Credenciais de teste e certificado Efí serão necessários para homologar o fluxo real.
A conciliação Pix atual funciona sem webhook público.

## Verificação

Executar testes de estados e preços, build do Go e do frontend. Validar autorização e
isolamento no backend. Migrações adicionam tabelas novas; não convertem dados existentes
nem removem empresas. O diretório `test/testutil` foi removido previamente neste checkout;
os testes antigos de handlers dependem dele e precisam de restauração separada para rodar.

Verificações realizadas nesta etapa:

- `go test ./internal/saas`: preços, limites, recorrência e vencimentos.
- `go test ./internal/saasapi`: integração com PostgreSQL descartável, autorização,
  isolamento de organizações, atualização sem duplicar assinatura e plano inativo.
  Requer `SAAS_TEST_DATABASE_URL`; cria esquema e usa transação com rollback.
- Build Go e build Docker com frontend Vue concluídos.
- Migrações confirmadas no banco local; endpoints novos rejeitam acesso sem autenticação.
