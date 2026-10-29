# K.O Gaming Store (Dev-Ops)

Loja de hardware gamer feita para a Avaliação Somativa N2 (Pipeline CI/CD + Monitoramento).

O frontend é em Svelte, a API em Go e o banco é MySQL. A pipeline roda no Azure DevOps: testa,
gera a imagem Docker e faz o deploy numa EC2 da AWS. Em produção o banco fica no RDS e as imagens
dos produtos no S3.

## Como funciona

```
git push -> GitHub -> Azure DevOps
                        |
                        +-- CI: go vet, testes (Go + MySQL), testes do frontend, build,
                        |       docker build, smoke test
                        +-- CD: copia a imagem pra EC2 via SCP, roda o deploy.sh via SSH
                                e confere o /healthz

EC2 (container lojagamer)
  API Go + frontend Svelte
  -> RDS MySQL (produtos, carrinho, pedidos)
  -> S3 (imagens)
```

É um container só: o binário Go serve a API e o frontend já compilado, e também repassa as
imagens do S3, então o bucket pode continuar privado.

## Pastas

```
backend/            API em Go
  cmd/server/         ponto de entrada (config, migrations, servidor)
  internal/api/       rotas, admin, carrinho, logs, métricas, /healthz
  internal/store/     consultas no MySQL
  internal/images/    imagens no disco local ou no S3
  internal/database/  conexão e migrations (schema + 50 produtos)
frontend/           Svelte 5 + Vite
  src/pages/          Home, Carrinho, Checkout, Admin
  src/components/     cards, busca, banner, etc.
seed-images/        imagens dos produtos iniciais (vão pro S3 no primeiro deploy)
deploy/             setup-ec2.sh, deploy.sh (com rollback) e .env.example
Dockerfile          build em etapas (Node, Go e imagem final distroless)
docker-compose.yml  ambiente local (MySQL + app)
azure-pipelines.yml pipeline
```

## Rodando localmente

Só precisa do Docker:

```bash
docker compose up --build
```

A loja abre em http://localhost:8080 e o admin em http://localhost:8080/admin (token `admin123`).
Também dá pra ver http://localhost:8080/healthz e http://localhost:8080/metrics.

### Rodando sem Docker (para desenvolver)

Precisa de Go 1.26+ e Node 22+. O MySQL pode continuar no Docker:

```bash
docker compose up -d mysql

cd backend
DB_PASSWORD=1234 ADMIN_TOKEN=admin123 go run ./cmd/server    # API na porta 8080

cd frontend
npm install
npm run dev                                    # Vite na 5173, repassa /api para a 8080
```

No PowerShell as variáveis ficam assim: `$env:DB_PASSWORD="1234"; $env:ADMIN_TOKEN="admin123"`.

## Testes

```bash
cd backend
go test ./...

# com os testes de integração no MySQL
TEST_MYSQL_DSN="root:1234@tcp(127.0.0.1:3306)/lojagamer_test?parseTime=true&multiStatements=true" go test -cover ./...

cd frontend
npm test              # ou npm run test:ci para ter cobertura
```

Para os testes de integração, crie antes o banco `lojagamer_test` (`CREATE DATABASE lojagamer_test;`).
Eles apagam e recriam as tabelas desse banco.

No backend os testes cobrem as rotas da API (usando um store em memória), o acesso ao MySQL real
(migrations, estoque, checkout e rollback), o armazenamento de imagens e a leitura da configuração.
No frontend cobrem o cliente da API, o roteador, os componentes e as páginas.

## API

| Método | Rota | O que faz |
|---|---|---|
| GET | `/api/products?on_sale=&random=&limit=` | lista produtos |
| GET | `/api/products/{id}` | busca um produto |
| POST / PUT / DELETE | `/api/products[/{id}]` | admin: cria, edita (multipart com `image`) e remove |
| GET | `/api/cart` | carrinho da sessão (cookie `lg_session`) |
| POST | `/api/cart/items` | `{"product_id": 1, "quantity": 1}` |
| PUT / DELETE | `/api/cart/items/{productID}` | muda a quantidade ou remove |
| POST | `/api/checkout` | `{"payment_method": "pix" \| "credito" \| "debito"}` |
| GET | `/images/{tipo}/{arquivo}` | imagem do produto |
| GET | `/healthz` | status, commit em execução, MySQL e S3 |
| GET | `/livez` | usado pelo HEALTHCHECK do Docker |
| GET | `/metrics` | métricas no formato do Prometheus |

As rotas de admin precisam do header `Authorization: Bearer <ADMIN_TOKEN>`.

## Variáveis de ambiente

Na EC2 elas ficam em `/opt/lojagamer/.env`. Tem um exemplo em [deploy/.env.example](deploy/.env.example).

- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`: conexão com o banco (padrão `localhost`, `3306`, `root`, sem senha, `lojagamer`)
- `DB_TLS`: `false` por padrão, no RDS usamos `preferred`
- `STORAGE_DRIVER`: `local` por padrão, `s3` em produção
- `S3_BUCKET`, `S3_PREFIX`, `AWS_REGION`: bucket das imagens (prefixo padrão `products/`, região `us-east-1`)
- `ADMIN_TOKEN`: senha do admin. Se ficar vazio, o admin fica desligado
- `MIGRATE`: roda as migrations ao subir (padrão `true`)
- `LOG_LEVEL`: `debug`, `info`, `warn` ou `error` (padrão `info`)

## Monitoramento

- Cada requisição gera um log em JSON com rota, status, duração e `request_id`. Dá pra ver com
  `docker logs -f lojagamer` ou mandar pro CloudWatch.
- O `/healthz` testa o MySQL e o S3 e mostra qual commit está rodando. A pipeline só considera o
  deploy ok quando o endereço público responde `ok` com o commit certo.
- O container tem HEALTHCHECK e o `deploy.sh` faz rollback: se a versão nova não subir saudável,
  a anterior volta e a pipeline falha.
- O `/metrics` mostra requisições por rota e status, latência, pedidos, faturamento e falhas de
  checkout e de health check.
- Os resultados dos testes e a cobertura aparecem nas abas Tests e Code Coverage do Azure DevOps.
