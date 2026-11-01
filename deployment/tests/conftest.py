"""Disposable proxy infrastructure, with cleanup registered before startup."""
import sys
from pathlib import Path
import pytest
from testcontainers.core.container import DockerContainer
from testcontainers.core.network import Network

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "deployment"))
CORE = """from http.server import BaseHTTPRequestHandler, HTTPServer
import json
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Cache-Control', 'no-store')
        self.end_headers()
        self.wfile.write(json.dumps({'path': self.path}).encode())
    do_POST = do_GET
    do_DELETE = do_GET
HTTPServer(('0.0.0.0', 8080), Handler).serve_forever()
"""


@pytest.fixture
def network(request):
    resource = Network()
    request.addfinalizer(resource.remove)
    resource.create()
    return resource


@pytest.fixture
def core(request, network):
    container = DockerContainer("nossa-inflacao-scrapers")
    container.with_network(network).with_network_aliases("core")
    container.with_command(["python", "-c", CORE])
    request.addfinalizer(container.stop)
    container.start()
    return container


@pytest.fixture
def web(request, core, network, tmp_path):
    config = (ROOT / "deployment/web/Caddyfile").read_text()
    config = config.replace("{$PUBLIC_DOMAIN} {", "http://{$PUBLIC_DOMAIN} {")
    config = config.replace("www.http://", "http://www.")
    path = tmp_path / "Caddyfile"
    path.write_text(config)
    path.chmod(0o644)
    container = DockerContainer("nossa-inflacao-web:verify")
    container.with_network(network).with_exposed_ports(80)
    container.with_env("PUBLIC_DOMAIN", "site.test")
    container.with_volume_mapping(str(path), "/etc/caddy/Caddyfile", mode="ro")
    container.with_kwargs(cap_drop=["ALL"], cap_add=["NET_BIND_SERVICE"])
    request.addfinalizer(container.stop)
    container.start()
    return container


@pytest.fixture
def scraper_browser(request):
    container = DockerContainer("nossa-inflacao-scrapers:verify")
    container.with_command(["sleep", "infinity"])
    container.with_kwargs(cap_drop=["ALL"], security_opt=["no-new-privileges:true"])
    request.addfinalizer(container.stop)
    container.start()
    return container
