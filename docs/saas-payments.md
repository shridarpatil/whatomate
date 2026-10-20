# Pagamentos SaaS: Mercado Pago e Efí

Implementado nesta etapa: renovação de plano por **Pix**, faturas por organização,
consulta periódica, cancelamento e reemissão. Mercado Pago é o principal;
Efí fica reservada para uma etapa futura; mantenha `EFI_CONTINGENCY=false`.
Asaas e Stripe não foram adicionados.

## Configurar no Docker local

A tela de faturas exibe o QR Code e o Pix Copia e Cola enquanto a cobrança está
pendente e dentro da validade. A imagem é gerada no navegador com o mesmo código
da fatura, sem serviços externos nem emissão de uma segunda cobrança. Se a imagem
não puder ser exibida, a tela orienta usar o Copia e Cola. O segredo do webhook
autentica notificações de pagamento; ele não é necessário para renderizar a imagem.

1. Adicione as variáveis de [`docker/payments.env.example`](../docker/payments.env.example)
   ao arquivo local `docker/.env`, preservando as variáveis já existentes.
2. Preencha `WHATOMATE_PAYMENTS__MP_ACCESS_TOKEN` com o token do ambiente escolhido.
   O exemplo do PlusZap usa `MP_ACCESS_TOKEN`; o Whatomate usa o nome com prefixo
   acima. Os arquivos `.env.exemple` não contêm as credenciais reais deste projeto.
3. Apenas quando for habilitar Efí futuramente, preencha client ID, client secret e chave Pix. Coloque o certificado
   PEM em `docker/payment-certs/efi.pem`. Ele deve conter também a chave privada,
   ou informe o caminho do segundo PEM em `WHATOMATE_PAYMENTS__EFI_PRIVATE_KEY`.
   O Compose monta essa pasta como `/run/payment-certs`, somente para leitura.
   P12/PFX precisam ser convertidos para PEM conforme a documentação da Efí.
4. Configure `WHATOMATE_PAYMENTS__EFI_SANDBOX=true` para homologação ou `false`
   para produção, usando as credenciais e o certificado correspondentes.
   Autorize os escopos `cob.read` e `cob.write` na aplicação Efí.
5. Depois de conferir as credenciais e o ambiente, defina
   `WHATOMATE_PAYMENTS__ENABLED=true`. Mantenha
   `WHATOMATE_PAYMENTS__EFI_CONTINGENCY=false` para Mercado Pago.
6. Recrie o aplicativo para carregar o ambiente:

   ```powershell
   docker compose -f docker/docker-compose.yml up -d --no-deps --force-recreate app
   ```

Para notificações imediatas, configure o webhook abaixo. A emissão de Pix e a
conciliação periódica funcionam mesmo sem webhook. A URL pode ser cadastrada no
painel do Mercado Pago ou enviada por cobrança usando
`WHATOMATE_PAYMENTS__MP_NOTIFICATION_URL`. Este é o equivalente a
`MP_NOTIFICATION_URL` do PlusZap. O segredo usa `WHATOMATE_PAYMENTS__MP_WEBHOOK_SECRET`.

### Túnel de teste existente

O Docker publica o Whatomate em `http://localhost:8050`, encaminhando para a
porta interna 8081. A publicação anterior em 8081 também continua disponível.
O túnel `https://meta.consigvip.com.br` pode continuar apontando para a porta 8050.
Foi adicionado o alias `POST /subscription/mercadopagowebhook`, que usa exatamente
a mesma validação de assinatura e baixa do endpoint `/api/billing/webhooks/mercadopago`.

Configuração local da URL:

```dotenv
WHATOMATE_PAYMENTS__MP_NOTIFICATION_URL=https://meta.consigvip.com.br/subscription/mercadopagowebhook
WHATOMATE_PAYMENTS__MP_WEBHOOK_SECRET=preencha_com_o_segredo_real_da_aplicacao
```

O segredo não é gerado pelo Whatomate. Use o segredo da aplicação Mercado Pago
associada ao access token. Sem ele o endpoint retorna 503 e a conciliação por
consulta continua funcionando. A URL é salva na fatura antes da emissão: alterações
posteriores na configuração não modificam o payload das tentativas anteriores.
Faturas antigas continuam por consulta; novas cobranças já recebem `notification_url`.

As credenciais ficam no servidor. A configuração completa habilita o botão de
emissão, mas **não comprova que o provedor aceitou as credenciais**. Erros de
autorização, certificado ou conexão aparecem ao consultar/emitir uma cobrança,
sem expor a resposta bruta do gateway nem suas chaves.

Fora do Docker, as mesmas configurações podem ser definidas em uma seção
`[payments]` do `config.toml`, com nomes como `enabled`, `mp_access_token`, `mp_webhook_secret`,
`efi_contingency`, `efi_client_id`, `efi_client_secret`, `efi_pix_key`,
`efi_certificate`, `efi_private_key` e `efi_sandbox`.

## Usar na tela

Em **Configurações → Planos e assinatura**, o superadministrador cria um plano
pago em BRL e vincula a organização. A assinatura precisa estar `active` ou
`trialing` (inclusive vencida); suspensões administrativas não são revertidas
automaticamente. O pagador usa o e-mail do usuário autenticado.

Usuários com `settings.general:write` podem gerar o Pix, copiar o código e
consultar/recuperar a cobrança. A leitura exige `settings.general:read`.
O backend obtém o preço do plano, sem aceitar valor ou status de pagamento do navegador.

A confirmação ocorre automaticamente por webhook autenticado ou consulta
periódica no servidor, mesmo com o navegador fechado. O worker inicia com o
aplicativo e verifica faturas pendentes a cada minuto, em lotes de até 100;
intervalos mínimos entre consultas, volume e indisponibilidade podem aumentar
o tempo de confirmação. Isso também funciona em localhost, sem URL pública.
A tela lê o estado local a cada 10 segundos enquanto está visível; o botão
Consultar é opcional. Nenhuma atualização da tela cria cobranças.

## Webhook do Mercado Pago

1. Publique esta versão do Whatomate no domínio público, com HTTPS válido.
2. Acesse **Mercado Pago Developers → Suas integrações → aplicação usada pelo
   access token → Webhooks**. Selecione o ambiente correto e o evento **Pagamentos**.
3. Cadastre `https://SEU-DOMINIO/api/billing/webhooks/mercadopago`. Se o domínio
   informado `chat.pluszap.com.br` já servir o Whatomate, use
   `https://chat.pluszap.com.br/api/billing/webhooks/mercadopago`.
   Se ele ainda servir o PlusZap antigo, publique primeiro a nova versão ou use
   outro domínio. Não substitua o webhook da aplicação antiga antes da migração.
4. Copie o segredo de assinatura dessa configuração para
   `WHATOMATE_PAYMENTS__MP_WEBHOOK_SECRET` no `docker/.env` do servidor Whatomate
   e recrie o container. Esse segredo é diferente do access token e nunca deve
   ser enviado ao frontend ou incluído na URL.
5. O proxy deve preservar query string, `x-signature` e `x-request-id` e encaminhar
   POSTs ao backend. Não direcione o webhook financeiro ao endpoint Meta `/api/webhook`.

O endpoint fica indisponível sem o segredo. Valida HMAC-SHA256 em tempo constante,
o identificador da query `data.id`, o identificador da requisição e o timestamp
(janela de 10 minutos; servidor deve ter relógio sincronizado). O corpo recebido
não determina valor, organização nem situação de pagamento. O sistema consulta
o pagamento na API autenticada e confere referência, ID, valor em centavos e moeda.

Webhooks e conciliação usam a mesma transação e trava por organização. Repetições
não renovam a assinatura duas vezes. Falhas transitórias retornam HTTP 503 para
permitir reentrega e o worker continua tentando. Divergências ficam em revisão,
sem conceder tempo de assinatura. Uma simulação com ID fictício do painel pode
retornar 503 porque nenhum pagamento correspondente existe na API; ela não comprova
baixa financeira. A confirmação completa deve usar um pagamento válido.

Um pagamento válido renova a assinatura uma única vez, a partir do maior valor
entre o vencimento atual e a data da confirmação. A soma de meses respeita o
último dia do mês. Alteração de plano, vencimento ou suspensão após a emissão
leva o pagamento à revisão, sem sobrescrever a decisão do administrador.

Se o código expirar, consulte seu estado e use **Cancelar este Pix**. Após
confirmação de cancelamento pelo gateway, **Reemitir cobrança cancelada** gera
uma nova cobrança. Se o pagamento já foi confirmado, o cancelamento consulta
e registra o pagamento, sem tentar estorná-lo.

## Acionar contingência

Defina `WHATOMATE_PAYMENTS__EFI_CONTINGENCY=true` e recrie o aplicativo.
Novas faturas usarão a Efí; as existentes continuam no gateway original.
Para voltar, use `false` e recrie novamente. Preserve as credenciais de ambos
enquanto existirem cobranças pendentes.

Uma falha de comunicação **não troca automaticamente o gateway**: o provedor
pode ter criado a cobrança mesmo sem retornar resposta. A recuperação mantém
o mesmo identificador e payload. O Mercado Pago permite recuperar por referência;
a Efí usa o mesmo `txid`. Uma reemissão só é permitida após cancelamento confirmado.
Essa regra também vale ao trocar de gateway para uma cobrança já emitida.

`ENABLED=false` impede novas emissões, mas mantém consultas/conciliação de
cobranças existentes. Os testes automatizados usam provedores simulados.

## Limites e validação

- Não inclui cartão, débito recorrente, Pix Automático, documentos fiscais,
  cálculo proporcional de mudança de plano ou gestão de estornos/chargebacks.
- Depois de liquidada, a fatura não é consultada continuamente. Estornos posteriores
  precisam ser tratados no provedor e revisados administrativamente.
- Faturas em revisão não são liberadas automaticamente; precisam de investigação
  antes de ajuste comercial. Não há botão para marcar uma fatura como paga.
- Limites dos planos ainda são informativos e não bloqueiam os recursos.
- A emissão de Pix no Mercado Pago foi verificada ao recuperar uma tentativa
  iniciada pelo administrador. Foi corrigido o formato de validade para incluir
  milissegundos e deslocamento explícito (`+00:00`), evitando o erro 400/código 23.
  O administrador confirmou pagamento real e renovação após consulta manual.
  A baixa sem interação e o webhook foram testados com API simulada e PostgreSQL;
  recebimento real de notificações depende da publicação/configuração acima.
  Estornos e integração real com a Efí continuam fora dessa validação.

Testes locais cobrem centavos sem arredondamento, idempotência, recuperação,
isolamento de organizações, contingência preservando o provedor original,
divergência de valor, cancelamento, reemissão e renovação única com PostgreSQL.
Também cobrem baixa pelo worker, assinatura adulterada/expirada, falha fechada,
webhooks repetidos e recuperação de uma resposta de emissão perdida.
Use exclusivamente um banco descartável em `SAAS_TEST_DATABASE_URL` e execute
`go test ./internal/billing` e `go test ./internal/saasapi` sequencialmente.

Referências oficiais:
[Mercado Pago — pagamentos](https://www.mercadopago.com.br/developers/pt/docs/subscriptions/additional-content/payment-management),
[Mercado Pago — webhooks](https://www.mercadopago.com.br/developers/pt/docs/subscriptions/additional-content/your-integrations/notifications/webhooks),
[Efí — credenciais e certificado](https://dev.efipay.com.br/docs/api-pix/credenciais/),
[Efí — cobranças imediatas](https://dev.efipay.com.br/docs/api-pix/cobrancas-imediatas/).
