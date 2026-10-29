# Inflação Real

**O preço mudou. A ideia é ter o histórico para entender quanto.**

Quem faz mercado conhece a sensação: o café ficou mais caro, o pacote parece
menor e a mesma lista de compras já não cabe no mesmo orçamento. Mas memória
de preço tem limite. Para acompanhar essas mudanças, precisamos de registros.

O **Inflação Real** é um projeto independente de monitoramento de preços do
varejo brasileiro. Coletamos ofertas de lojas online e guardamos seu histórico,
com a origem e as condições de cada preço. O objetivo é transformar esses dados
em informações que ajudem a entender o custo das compras e o poder de compra
ao longo do tempo.

## O que queremos responder

Quem trabalha e paga as contas não deveria depender do Estado para entender
o que o próprio dinheiro compra. Informação independente dá às pessoas mais
autonomia para avaliar o próprio orçamento e questionar o discurso de quem governa.

- Os índices oficiais de inflação refletem a perda de poder de compra que
  o brasileiro sente nas compras do dia a dia?
- Quanto o seu dinheiro compra hoje em comparação com o início do período?
- Quanto custa hoje a mesma lista de compras, com os mesmos produtos e quantidades?
- Como conferir essas mudanças por conta própria, sem precisar aceitar a
  interpretação do governo?

Os números do Estado devem estar abertos ao escrutínio de quem paga a conta.
Queremos dar ferramentas para isso: histórico consultável, origem dos preços
e critérios claros, para que cada pessoa possa tirar suas próprias conclusões.

Essa independência também exige rigor. O preço de algumas lojas, sozinho,
não representa a inflação do país. Comparações com índices oficiais precisam
explicar o período, a cobertura e as diferenças de metodologia. Se os dados
contrariarem a nossa expectativa, a conclusão precisa acompanhar os dados.

## Onde o projeto está hoje

Estamos construindo a base de coleta e histórico.

| Parte | O que já existe |
| --- | --- |
| Scrapers em Python | Adapters para Super Muffato, Angeloni e Festval, com configurações de catálogo online na região de Curitiba. |
| Core em Go | API autenticada de ingestão, validação, fila de processamento, identificação de produtos e persistência do histórico. |
| PostgreSQL | Migrations automáticas e armazenamento de produtos, ofertas e origem das observações. |
| Interface em React | Planejada; ainda sem implementação. |

O escopo de monitoramento usa os grupos de alimentos da cesta básica do DIEESE
como referência, com diferenças documentadas. A coleta ainda se concentra em
produtos embalados: itens vendidos por peso, como carnes e hortifruti, precisam
de suporte adicional. Cadastrar um grupo de alimentos não significa que ele já
tenha cobertura completa.

Ainda não há um índice de inflação calculado pelo projeto nem um painel público.
Os detalhes do recorte estão na [documentação de cobertura](docs/contracts/monitoring/v1/basic-food-basket.md).

## Como funciona

```text
Lojas online → Scrapers Python → Core Go ↔ PostgreSQL
                                   ↕
                             Interface React
                               (planejada)
```

Os scrapers consultam as lojas e enviam observações no mesmo
[formato versionado](docs/contracts/ingestion/v1/README.md). O core valida os
envios, processa os batches e grava o histórico. As regras de negócio e os cálculos
ficam no core; a interface vai apresentar os dados pela API.

Alguns cuidados fazem parte dessa base:

- **Preço sem contexto engana.** Loja, data, região e condições da oferta
  acompanham a observação quando estão disponíveis.
- **Ausência de preço não é preço zero.** Informação desconhecida continua
  desconhecida.
- **Embalagem e variante importam.** Marcas, tamanhos e versões diferentes
  mantêm identidades próprias.
- **Cada número precisa ter uma origem.** Valores monetários usam representação
  decimal exata, e as observações preservam sua procedência.

## Rodando localmente

Você vai precisar de Docker com Compose, Go **1.26+** para o core e Python
**3.12+** para os scrapers. Por enquanto, o Compose sobe apenas o PostgreSQL;
core e scrapers rodam na máquina.

Na raiz do repositório, crie a configuração local se ela ainda não existir:

```sh
test -f .env || cp .env.example .env
```

Edite o `.env` com sua senha em `POSTGRES_PASSWORD` e um token próprio em
`INGESTION_TOKEN`. Configure também `INGESTION_SCRAPER_ID` e
`INGESTION_SHOP_IDS` de acordo com os scrapers que pretende usar. O arquivo
é ignorado pelo Git.

Suba o banco e aguarde o health check:

```sh
docker compose up -d --wait
```

Depois, inicie o core:

```sh
cd core
go run ./cmd/core -env-file ../.env
```

As migrations são aplicadas na inicialização. Por padrão, a API fica em
`http://127.0.0.1:8080` e o banco em `127.0.0.1:5432`. Se a porta do banco
estiver ocupada, ajuste `POSTGRES_PORT` no `.env`.

Para começar a coletar, siga o [guia dos scrapers](scrapers/README.md).
Ele inclui a instalação do ambiente Python, os arquivos de configuração e o
cadastro das lojas no core. O token e a identidade do scraper precisam coincidir
com a configuração do core, e as lojas precisam estar cadastradas antes do
processamento das observações.

Para consultar o banco sem instalar um cliente PostgreSQL na máquina, execute
na raiz do repositório:

```sh
docker compose exec postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

`docker compose down` encerra o banco e preserva os dados. A opção `--volumes`
apaga o banco local. Alterar a senha no `.env` não troca a senha de um banco
que já foi inicializado.

## Como contribuir

Tem espaço para quem gosta de coleta de dados, Go, interfaces ou de investigar
por que dois preços aparentemente iguais contam histórias diferentes.

Algumas contribuições úteis neste estágio:

- Melhorar os scrapers e adicionar lojas, com exemplos de resposta e testes
  que possam rodar sem acessar o site real.
- Investigar erros de identificação de produtos, embalagens e condições de oferta.
- Ampliar o suporte a formatos de preço ainda não atendidos, como venda por peso.
- Construir a primeira visualização de histórico, com linguagem simples e acessibilidade.
- Revisar a documentação, os critérios de coleta e as limitações dos dados.

Para mudanças maiores, abra uma issue primeiro para discutir o problema e o
recorte. Para correções pequenas, pode mandar um pull request direto.

1. Faça um fork e crie uma branch para sua alteração.
2. Leia o [guia do projeto](AGENTS.md) e o `AGENTS.md` do componente que vai alterar.
3. Mantenha a mudança focada. Prefira uma solução simples que funcione de ponta a ponta.
4. Rode as verificações do componente e atualize a documentação afetada.
5. No pull request, explique o problema, o que mudou e como verificou o resultado.

Mudanças no contrato de dados precisam atualizar os consumidores afetados juntos.
Credenciais, tokens e configurações pessoais ficam fora dos commits.

Se encontrou um preço estranho, uma issue com loja, link do produto, data e
condições da oferta já ajuda bastante. Evite incluir dados pessoais, cookies ou
tokens de sessão.

### Verificações

No diretório `core/`:

```sh
go test ./...
go vet ./...
```

O Docker precisa estar disponível. Os testes de integração criam bancos
descartáveis com Testcontainers e fazem a limpeza automaticamente; não usam
o banco de desenvolvimento.

No diretório `scrapers/`, depois de instalar as dependências de desenvolvimento
conforme o guia do componente:

```sh
.venv/bin/pytest
.venv/bin/ruff check src tests
.venv/bin/ruff format --check src tests
```

Os testes padrão dos scrapers rodam sem banco, Docker ou acesso às lojas.
As verificações contra os sites reais são opcionais e estão descritas no guia.

## Para se aprofundar

- [Core: configuração, API e processamento](core/README.md)
- [Scrapers: instalação, lojas e envio de observações](scrapers/README.md)
- [Contrato de ingestão](docs/contracts/ingestion/v1/README.md)
- [Regras compartilhadas de monitoramento](docs/contracts/monitoring/v1/README.md)
- [Escopo da cesta básica e limites de cobertura](docs/contracts/monitoring/v1/basic-food-basket.md)
