"""Build before stopping collection; migrate and probe before scheduling."""
from runtime import compose, revision, CONFIG, require_clean_checkout
from settings import bootstrap, initialize

SHOPS = """INSERT INTO shops (id, name, website_url) VALUES
('super-muffato','Super Muffato','https://www.supermuffato.com.br'),
('angeloni','Angeloni','https://super.angeloni.com.br'),
('festval','Festval','https://www.festval.com') ON CONFLICT (id) DO NOTHING;"""


def database(*arguments, **options):
    return compose("exec", "-T", "postgres", *arguments, **options)


def sql(query):
    return database("psql", "-U", "nossa_inflacao", "-d", "nossa_inflacao",
                    "-v", "ON_ERROR_STOP=1", input=query.encode())


def start():
    require_clean_checkout()
    bootstrap()
    initialize()
    compose("build", "--pull")
    compose("stop", "scrapers")
    start_application()
    (CONFIG / "revision").write_text(revision() + "\n")


def start_application():
    compose("up", "-d", "--wait", "--wait-timeout", "180", "postgres", "core")
    sql(SHOPS)
    compose("up", "-d", "scrapers")
    compose("ps")


def scraper_command(action):
    compose("stop", "scrapers")
    try:
        compose("run", "--rm", "--no-deps", "scrapers", "inflacao-scraper",
                "--config", "/app/config.toml", action)
    finally:
        compose("start", "scrapers")
