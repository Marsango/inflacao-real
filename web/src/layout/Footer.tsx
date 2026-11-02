import { GitHubIcon } from './GitHubIcon';

export function Footer() {
  return <footer><div className="page-width footer-content"><span>Nossa inflação · Projeto independente</span>
    <nav className="footer-links" aria-label="Sobre a Nossa inflação">
      <a href="/sobre#metodologia">Sobre os dados e a metodologia</a>
      <a href="/sobre#projeto">Sobre o projeto</a>
      <a href="https://github.com/Marsango/nossa-inflacao"><GitHubIcon />Contribua para o projeto</a>
    </nav></div></footer>;
}
