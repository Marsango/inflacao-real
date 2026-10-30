"""Optional public website configuration and startup checks."""
import re
from runtime import CONFIG, command, compose, production_domain


def validate_domain(domain):
    domain = domain.lower()
    label = r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?"
    if len(domain) > 253 or not re.fullmatch(rf"{label}(?:\.{label})+", domain):
        raise ValueError("Use a DNS hostname without a scheme, port, path, or trailing dot")
    if domain.startswith("www.") or not re.search(r"[a-z]", domain.rsplit(".", 1)[1]):
        raise ValueError("Use the root domain, such as nossainflacao.com.br")
    return domain


def save_domain(domain):
    if domain is None:
        return
    domain = validate_domain(domain)
    path = CONFIG / "production.env"
    lines = [line for line in path.read_text().splitlines() if not line.startswith("PUBLIC_DOMAIN=")]
    temporary = CONFIG / "production.env.pending"
    temporary.write_text("\n".join([*lines, "PUBLIC_DOMAIN=" + domain]) + "\n")
    temporary.chmod(0o600)
    temporary.replace(path)


def start_website():
    if not production_domain():
        return
    compose("exec", "-T", "core", "curl", "--fail", "--silent", "--output", "/dev/null",
            "http://127.0.0.1:8080/api/v1/products?limit=1")
    command("ufw", "allow", "80/tcp")
    command("ufw", "allow", "443/tcp")
    compose("up", "-d", "--wait", "--wait-timeout", "120", "web")
    print("Website configured at https://" + production_domain() +
          "; DNS must point here for automatic HTTPS. Inspect ./deploy logs for TLS errors.")
