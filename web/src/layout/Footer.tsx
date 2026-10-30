import type { History } from '../data/model';
import { GitHubIcon } from './GitHubIcon';

export function Footer({ history }: { history?: History }) {
  return <footer><div className="page-width footer-content"><span>Nossa inflação · Projeto independente</span>
    <div className="footer-links">
      <details><summary>Sobre os dados e a metodologia</summary><div className="footer-description">
        <p>{history?.coverage ?? 'Monitoramento de preços do varejo brasileiro, com origem e condições de cada observação.'}</p>
        <p>{history?.methodology ?? 'Preços, cobertura e metodologia são fornecidos pelo core para o intervalo selecionado. O recorte monitorado não representa a inflação nacional.'}</p>
        <p>Ausência de preço comparável não é preço zero. O gráfico mantém uma lacuna quando a cobertura é incompleta ou as condições não permitem comparação.</p>
      </div></details>
      <details><summary>Sobre o projeto</summary><div className="footer-description">
        <p>A Nossa inflação é um projeto independente que acompanha preços do varejo brasileiro para ajudar as pessoas a entender as mudanças no custo das compras.</p>
        <p>Preservamos a origem e as condições dos preços. Nossas conclusões devem acompanhar as evidências coletadas.</p>
      </div></details>
      <a href="https://github.com/Marsango/nossa-inflacao"><GitHubIcon />Contribua para o projeto</a>
    </div></div></footer>;
}
