"""Initialize secrets once; changes remain explicit and outside the checkout."""
import secrets
from runtime import CONFIG, ROOT, command


def initialize():
    CONFIG.mkdir(mode=0o700, parents=True, exist_ok=True)
    write_once("production.env", "POSTGRES_PASSWORD=" + secrets.token_hex(32) +
               "\nINGESTION_TOKEN=" + secrets.token_hex(32) + "\nCOLLECTION_TIME=09:00\n")
    template = (ROOT / "scrapers/scrapers.example.toml").read_text()
    write_once("scrapers.toml", template)
    migrate_internal_address()


def write_once(filename, content):
    path = CONFIG / filename
    if path.exists():
        return
    with path.open("x") as destination:
        destination.write(content)
    path.chmod(0o600 if filename.endswith(".env") else 0o644)


def bootstrap():
    command("bash", str(ROOT / "deployment/bootstrap.sh"))


def migrate_internal_address():
    path = CONFIG / "scrapers.toml"
    original = path.read_text()
    updated = original.replace('core_url = "http://core:8080"',
                               'core_url = "http://127.0.0.1:8080"')
    if updated != original:
        path.write_text(updated)
