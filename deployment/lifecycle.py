"""Build before stopping collection; migrate and probe before scheduling."""
from runtime import compose, revision, CONFIG, require_clean_checkout
from settings import bootstrap, initialize, synchronize_scrapers
from website import save_domain, start_website, validate_domain

SHOPS = """INSERT INTO shops (id, name, website_url) VALUES
('super-muffato','Super Muffato','https://www.supermuffato.com.br'),
('angeloni','Angeloni','https://super.angeloni.com.br'),
('festval','Festval','https://www.festval.com'),
('mambo','Mambo','https://www.mambo.com.br'),
('hirota','Hirota','https://www.lojahirota.com.br'),
('pao-de-acucar','Pão de Açúcar','https://www.paodeacucar.com'),
('sonda','Sonda','https://www.sondadelivery.com.br'),
('nagumo','Nagumo','https://www.nagumo.com.br') ON CONFLICT (id) DO NOTHING;"""


def database(*arguments, **options):
    return compose("exec", "-T", "postgres", *arguments, **options)


def sql(query):
    return database("psql", "-U", "nossa_inflacao", "-d", "nossa_inflacao",
                    "-v", "ON_ERROR_STOP=1", input=query.encode())


def start(domain=None):
    if domain is not None:
        validate_domain(domain)
    require_clean_checkout()
    bootstrap()
    initialize()
    save_domain(domain)
    compose("build", "--pull")
    compose("stop", "scrapers")
    synchronize_scrapers()
    start_application()
    (CONFIG / "revision").write_text(revision() + "\n")


def start_application():
    compose("up", "-d", "--wait", "--wait-timeout", "180", "postgres", "core")
    sql(SHOPS)
    compose("up", "-d", "--force-recreate", "scrapers")
    start_website()
    compose("ps")


def scraper_command(action):
    compose("stop", "scrapers")
    try:
        compose("run", "--rm", "--no-deps", "scrapers", "inflacao-scraper",
                "--config", "/app/config.toml", action)
    finally:
        compose("start", "scrapers")
