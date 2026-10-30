"""Fixed production identity and subprocess boundaries; never load local .env."""
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
CONFIG = Path(os.environ.get("NOSSA_CONFIG_DIR", "/etc/nossa-inflacao"))


def command(*arguments, **options):
    return subprocess.run(arguments, check=True, **options)


def compose(*arguments, **options):
    domain = production_domain()
    environment = {**os.environ, "NOSSA_CONFIG_DIR": str(CONFIG), "PUBLIC_DOMAIN": domain}
    profiles = []
    if domain:
        profiles = ["--profile", "public"]
    return command("docker", "compose", "--project-name", "nossa-inflacao",
                   "--env-file", str(CONFIG / "production.env"),
                   "-f", str(ROOT / "deployment/compose.yaml"), *profiles, *arguments,
                   env=environment, **options)


def revision():
    result = command("git", "--no-optional-locks", "-c", "safe.directory=" + str(ROOT), "-C", str(ROOT), "rev-parse", "HEAD", capture_output=True, text=True)
    return result.stdout.strip()


def require_clean_checkout():
    result = command("git", "--no-optional-locks", "-c", "safe.directory=" + str(ROOT), "-C", str(ROOT), "status", "--porcelain", capture_output=True, text=True)
    if result.stdout.strip():
        raise RuntimeError("Commit deployment changes first; production builds require a clean checkout")


def spool_archive(*arguments, **options):
    result = compose("images", "-q", "scrapers", capture_output=True, text=True)
    image = result.stdout.strip()
    if not image:
        raise RuntimeError("Build the scraper image before accessing its queue archive")
    return command("docker", "run", "--rm", "-i", "--network", "none",
                   "--user", "10001:10001", "--cap-drop=ALL",
                   "--security-opt=no-new-privileges", "-v",
                   "nossa-inflacao_scraper_data:/app/data", image, "tar", *arguments,
                   **options)


def production_domain():
    path = CONFIG / "production.env"
    if not path.exists():
        return ""
    for line in path.read_text().splitlines():
        if line.startswith("PUBLIC_DOMAIN="):
            return line.partition("=")[2].strip()
    return ""
