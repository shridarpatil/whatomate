# S3 local para gravações

O Compose inclui RustFS (API compatível com S3), volume persistente `s3-data`
e criação automática do bucket privado `whatomate-recordings`.

Na raiz do projeto:

```powershell
docker compose -f docker/docker-compose.yml up -d rustfs s3-init
docker compose -f docker/docker-compose.yml up -d --build --no-deps app
```

Configuração de desenvolvimento em `config.toml` (arquivo não versionado):

```toml
[storage]
type = "local"
local_path = "./uploads"
s3_bucket = "whatomate-recordings"
s3_region = "us-east-1"
s3_key = "whatomate-local"
s3_secret = "whatomate-local-dev-secret"
s3_endpoint = "http://rustfs:9000"
s3_public_endpoint = "http://localhost:9000"
s3_use_path_style = true

[calling]
recording_enabled = true
```

Mescle esses campos nas seções existentes. Os uploads comuns continuam locais;
as gravações usam o cliente S3 independentemente de `storage.type`.
Se executar o Go diretamente no Windows, use `http://localhost:9000` também
em `s3_endpoint`. Em Docker, `rustfs` é o hostname interno do serviço.
O endpoint público é usado antes da assinatura para que os links funcionem
no navegador da máquina local.

- API S3: http://localhost:9000
- Painel: http://localhost:9001/rustfs/console/
- Usuário: `whatomate-local`
- Senha: `whatomate-local-dev-secret`

Essas credenciais são exclusivas para desenvolvimento local. As portas do
armazenamento ficam vinculadas a `127.0.0.1`. O bucket permanece privado e
as gravações são acessadas por links temporários. O volume preserva os dados
entre reinicializações; `docker compose down -v` apaga os volumes.

Teste de upload e download por URL assinada, no PowerShell:

```powershell
$env:WHATOMATE_TEST_S3_ENDPOINT = 'http://localhost:9000'
go test ./internal/storage -run TestS3Client_LocalIntegration -v
Remove-Item Env:WHATOMATE_TEST_S3_ENDPOINT
```

O teste grava `diagnostics/s3-test.txt`, que pode ser removido pelo painel.
Para validar áudio real, faça uma chamada de teste e reproduza sua gravação
no histórico de ligações.

Referências: [RustFS em containers](https://docs.rustfs.com/en/installation/container)
e [endpoints do SDK AWS](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html).
