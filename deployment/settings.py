"""Preserve secrets/settings; append newly supported collectors during deployment."""
import re
import secrets
import tomllib
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


def synchronize_scrapers():
    path = CONFIG / "scrapers.toml"
    original = path.read_text()
    template = (ROOT / "scrapers/scrapers.example.toml").read_text()
    additions = missing_scraper_blocks(original, template)
    if not additions:
        return
    updated = original.rstrip() + "\n\n" + "\n\n".join(additions) + "\n"
    tomllib.loads(updated)
    path.write_text(updated)


def configured_adapters(document):
    entries = document.get("scrapers")
    if not isinstance(entries, list):
        raise ValueError("Expected [[scrapers]] entries in the deployment configuration")
    return {entry.get("adapter", "super-muffato") for entry in entries}


def missing_scraper_blocks(original, template):
    existing = configured_adapters(tomllib.loads(original))
    blocks = re.split(r"(?m)^(?=\[\[scrapers\]\])", template)
    return [block.strip() for block in blocks
            if block.startswith("[[scrapers]]") and
            missing_scraper_block(block, existing)]



def missing_scraper_block(block, existing):
    document = tomllib.loads(block)
    adapters = configured_adapters(document)
    return not adapters.issubset(existing)
