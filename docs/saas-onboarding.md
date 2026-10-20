# Cadastro de empresas com teste

O superadministrador pode cadastrar um cliente em **Configurações → Planos e
assinatura → Nova empresa em teste**.

1. Crie ou edite um plano ativo, com `Dias de teste` maior que zero e limite de
   usuários de pelo menos um (ou `-1`, ilimitado).
2. Informe o nome da empresa, nome e e-mail de seu administrador e uma senha
   inicial de pelo menos 12 caracteres (máximo de 72 bytes).
3. Escolha o plano e clique em **Criar empresa com teste**.
4. Entregue as credenciais ao responsável por um canal seguro. Ele já poderá
   entrar na tela de login. Nenhum e-mail é enviado automaticamente.

A criação salva em uma única transação a organização, os perfis de permissão,
o administrador, seu vínculo, as configurações iniciais, o painel, a assinatura
`trialing` e a auditoria. O prazo é calculado no servidor a partir dos dias do
plano. Não é necessário definir o vencimento manualmente.

O administrador criado não é superadministrador. Seu e-mail precisa ser novo;
contas existentes não são reaproveitadas e suas senhas não são alteradas.
Repetir o cadastro com o mesmo e-mail é recusado, sem criar outra organização.
O formulário limpa a senha depois de cada tentativa.

As empresas existentes não recebem trial automaticamente. Para elas continua
disponível o vínculo manual no painel comercial. Esta etapa não cria um
cadastro público de empresas nem concede testes por acesso anônimo.

## Isolamento

- Apenas o superadministrador acessa `POST /api/saas/organizations`.
- A listagem de organizações de usuários comuns fica restrita às suas associações.
- O cadastro público antigo por `organization_id` não admite novos membros em
  organizações com assinatura SaaS. O administrador deve criar/adicionar membros
  pelas telas autenticadas existentes.
- Usuários ligados a uma organização SaaS não podem criar empresas sem plano
  pelo cadastro antigo de organizações. Novas empresas passam pelo operador.

## Vencimento e pagamentos

O plano aparece como expirado quando o teste termina. A renovação por Pix usa
o [fluxo de pagamentos](saas-payments.md). **Ainda não existe bloqueio automático
dos recursos por expiração nem aplicação dos limites**; essa é uma etapa
separada da migração. O cadastro e o vencimento do teste não geram cobrança.

Testes com PostgreSQL descartável verificam permissões, associação do responsável,
senha armazenada como hash bcrypt, prazo, repetição por e-mail, isolamento e rollback
com falha injetada após a criação do usuário. Executar
`go test ./internal/saas ./internal/saasapi` com `SAAS_TEST_DATABASE_URL` apontando
para um banco de teste.
