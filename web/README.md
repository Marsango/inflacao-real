# Interface da Nossa inflação

Dashboard em React, TypeScript e Vite, integrado ao core Go pela
[API web v1](../docs/contracts/web/v1/README.md). Produtos, séries, variações,
cobertura, condições de preço e metodologia vêm do backend.

## Desenvolvimento

Use Node.js 20.19+ ou 22.12+ e npm. Inicie o PostgreSQL e o core conforme o
[guia principal](../README.md#rodando-localmente), com observações processadas
pelos scrapers para ter histórico disponível.

```sh
cd web
npm ci
npm run dev
```

Abra o endereço local mostrado pelo Vite. O servidor de desenvolvimento encaminha
`/api` para `http://127.0.0.1:8080`; se o core usar outra porta, ajuste o alvo em
`vite.config.ts`. O navegador usa caminhos relativos e não recebe o token de
ingestão. Não há dados fictícios nem cenários de demonstração em tempo de execução.
Um banco vazio mostra os estados vazios da aplicação.

## Verificação

```sh
npm run typecheck
npm test
npm run build
```

O build estático fica em `dist/`. Não há comando de lint configurado. Os testes
interceptam `fetch` com respostas do contrato e não usam banco ou lojas reais.
Cobrem catálogo paginado, filtros, valores do core, formatação brasileira,
carregamento, falha, recuperação, ausência de dados, cobertura insuficiente,
frescor, cancelamento e lacunas no gráfico. O build usa `/api` na mesma origem. Para hospedar com HTTPS no VPS,
veja o [guia de implantação](../docs/deployment.md#public-frontend-and-https).

## Interações e dados

Categoria, produto e intervalo ficam na URL. A seleção de produtos vem do catálogo canônico
e mantém marcas, embalagens e variantes separadas. A visão geral usa
`product=global`; produtos usam IDs decimais como strings.

As categorias Cesta básica, Hortifruti, Doces e Carnes vêm de `/api/v1/groups`.
Seus botões filtram a visão geral e a busca de produtos, preservando o período.
Trocar de categoria volta à visão geral; “Todos os produtos” remove esse filtro.
A composição aparece abaixo dos botões. Cesta básica é uma seleção com pesos
iguais por produto, não o custo ou índice de uma cesta oficial. Produtos sem
classificação continuam disponíveis em “Todos os produtos”. As categorias têm
carregamento e recuperação de erros independentes do histórico.

Os botões de período usam meses de calendário, terminando no dia atual em São
Paulo. Datas no fim do mês são ajustadas para o último dia válido do mês de
destino. Alterar uma data ativa o intervalo personalizado. Trocar o produto
preserva o intervalo. Datas inválidas, invertidas ou separadas por mais de 366
dias não disparam requisições.

Valores monetários são centavos inteiros para a base informada (embalagem, kg,
L ou unidade); porcentagens são pontos-base inteiros (803 = 8,03%). Datas são
dias de calendário em São Paulo; timestamps de coleta incluem fuso. Valores
ausentes são `null`. O navegador apenas apresenta os indicadores calculados
pelo core e mantém lacunas quando falta um preço selecionado.

A tabela fica em um controle expansível abaixo do gráfico. O core limita a série
à primeira e à última data com preço elegível dentro da seleção. Quando a seleção
começa antes ou termina depois dos dados, um aviso abaixo do gráfico informa as
datas disponíveis. As datas selecionadas e a URL são preservadas. O resumo mostra
as datas efetivamente usadas quando existe uma comparação válida. Um único dia
aparece como ponto, sem variação; na visão geral, esse ponto é apenas a base de
referência. Intervalos inteiramente sem dados continuam vazios.
Condições de compra não informadas aparecem em um aviso: preço anunciado não
significa preço disponível sem restrições. A cobertura, a última coleta e a
metodologia específica (em “Metodologia deste histórico”) ficam junto ao gráfico.
A página `/sobre`, acessível pelo rodapé, explica o projeto e a metodologia geral.
A comparação usa contextos com preços nas duas datas, com pesos
iguais por loja e por produto na visão geral. A tabela por loja mostra também
preços de contextos excluídos, com lacunas explícitas. Ela apresenta somente a
primeira e a última data do intervalo efetivo (uma coluna para um único dia),
independentemente do tamanho da série. Todas as ofertas retornadas continuam em
linhas separadas. Os rótulos mostram localidade, CEP e modalidade de compra, sem
identificadores internos de loja ou vendedor. “Ver oferta” abre a página do
produto na loja em uma nova aba. Em telas pequenas, a tabela permite rolagem
horizontal com foco pelo teclado; a série consolidada completa continua em
“Ver dados em tabela”. O resumo mostra o menor
preço disponível na última data com preço, separado da média do painel no gráfico.
Sem painel comparável, o gráfico de produto mostra o menor preço disponível por
dia e explica que mudanças de loja ou contexto não permitem calcular uma variação.

`src/data/api.ts` centraliza transporte, paginação e erros. Os hooks abortam
requisições obsoletas e oferecem tentativas independentes para catálogo e
histórico. Não há cálculo econômico no frontend.

## Dependências

React é mantido pela Meta; TypeScript, pela Microsoft. Vite/Vitest,
Testing Library e jsdom são projetos estabelecidos de código aberto, usados
somente no desenvolvimento e nos testes. As versões ficam fixadas no lockfile.
Os controles usam HTML nativo e o gráfico usa SVG próprio: sem biblioteca de
componentes, ícones, datas ou gráficos e sem fontes externas. A integração não
adiciona dependências.

## Implantação

Na raiz do repositório, `./deploy up --domain nossainflacao.com.br` habilita a
interface pública; atualizações usam `./deploy up`. Caddy serve o build estático,
gerencia HTTPS e encaminha a API pública de leitura e as rotas administrativas
explicitamente permitidas. O token de ingestão
permanece no backend. Configure os registros DNS conforme o guia de implantação.


## Administração de equivalências

A rota `/admin` permite login, revisão exata/aproximada, busca manual, inspeção e
reversão de grupos. O core exige `ADMIN_USERNAME` e `ADMIN_PASSWORD_HASH`; sem
credenciais, a API administrativa fica desativada. Use HTTPS também no ambiente
de desenvolvimento: os cookies de sessão são sempre seguros. Regras genéricas
iniciam desativadas; revise a prévia antes da ativação. Veja o
[contrato administrativo](../docs/contracts/web/v1/admin.md) e o
[guia de implantação](../docs/deployment.md#product-equivalence-administration).
