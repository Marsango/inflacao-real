import { Footer } from '../layout/Footer';
import { Header } from '../layout/Header';

function Project() {
  return <section id="projeto" aria-labelledby="project-title"><h2 id="project-title">Sobre o projeto</h2>
    <p>A Nossa inflação é um projeto independente que acompanha preços do varejo brasileiro para ajudar as pessoas a entender as mudanças no custo das compras.</p>
    <p>Preservamos a origem e as condições dos preços. Nossas conclusões devem acompanhar as evidências coletadas.</p>
    <p>O recorte monitorado não representa a inflação nacional. A cobertura depende das lojas, localidades e produtos acompanhados em cada período.</p>
  </section>;
}

function Methodology() {
  return <section id="metodologia" aria-labelledby="methodology-title"><h2 id="methodology-title">Sobre os dados e a metodologia</h2>
    <p>Monitoramos preços anunciados em lojas online, preservando a origem, a data e as condições de cada observação. Os valores podem variar conforme a localidade, a modalidade de compra e as condições da oferta.</p>
    <h3>Como ler o histórico</h3>
    <p>O intervalo efetivo vai da primeira à última data com preço elegível dentro do período selecionado. Ausência de preço comparável não é preço zero: o gráfico mantém uma lacuna quando faltam dados ou condições para a comparação.</p>
    <p>O resumo de um produto mostra o menor preço disponível na última data com preço. A comparação entre lojas mostra as ofertas nas duas pontas do intervalo, inclusive aquelas que ficaram fora do cálculo da variação.</p>
    <h3>Como calculamos a variação</h3>
    <p>Comparamos ofertas com preços nas duas datas e no mesmo contexto de compra. Cada loja tem o mesmo peso; na visão geral, cada produto também tem o mesmo peso. Sem ofertas comparáveis, o gráfico do produto mostra o menor preço disponível por dia, sem calcular uma variação.</p>
    <p>Preços com condições não informadas podem exigir requisitos não verificados. Quando isso ocorre, o histórico apresenta um aviso.</p>
    <h3>Confira a cobertura de cada consulta</h3>
    <p>A cobertura, a última coleta e a metodologia específica aparecem junto ao histórico selecionado. Consulte “Metodologia deste histórico” abaixo do gráfico para conhecer as regras e a versão usadas no cálculo.</p>
  </section>;
}

export function About() {
  return <><Header /><main className="page-width about-page">
    <a className="back-link" href="/">← Explorar preços</a>
    <h1>Sobre a Nossa inflação</h1>
    <p className="about-intro">Entenda o projeto, de onde vêm os preços e o que os dados permitem comparar.</p>
    <Project /><Methodology />
  </main><Footer /></>;
}
