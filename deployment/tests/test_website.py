import time

import pytest
import requests
import runtime
import settings
import website


def request(web, path="/", method="GET", host="site.test"):
    address = f"http://{web.get_container_host_ip()}:{web.get_exposed_port(80)}{path}"
    return request_when_ready(address, method, host, 30)


def request_when_ready(address, method, host, attempts):
    try:
        return requests.request(method, address, headers={"Host": host}, timeout=3,
                                allow_redirects=False)
    except requests.ConnectionError:
        return retry_request(address, method, host, attempts)


def retry_request(address, method, host, attempts):
    if attempts == 0:
        raise AssertionError("Website never became ready")
    time.sleep(1)
    return request_when_ready(address, method, host, attempts - 1)


def test_static_and_proxy_boundaries(web):
    page = request(web)
    assert page.status_code == 200
    assert "text/html" in page.headers["Content-Type"]
    assert page.headers["Cache-Control"] == "no-cache"
    assert request(web, "/some/frontend/path").text == page.text
    assert request(web, "/assets/missing.js").status_code == 404
    check_private_routes(web)
    check_public_routes(web)
    check_asset_headers(web, page.text)
    redirect = request(web, "/?product=12", host="www.site.test")
    assert redirect.status_code == 301
    assert redirect.headers["Location"] == "https://site.test/?product=12"


def check_private_routes(web):
    for path in ["/api", "/api/v1/unknown", "/api/v1/monitoring-targets",
                 "/api/v1/ingestion/submissions"]:
        assert request(web, path).status_code == 404
    assert request(web, "/api/v1/ingestion/submissions", "POST").status_code == 404
    assert request(web, "/api/v1/products", "POST").status_code == 404


def check_public_routes(web):
    for path in ["/api/v1/products?limit=1", "/api/v1/history?product=global&start=2026-10-01&end=2026-10-30"]:
        response = request(web, path)
        assert response.status_code == 200
        assert response.json()["path"] == path
        assert response.headers["Cache-Control"] == "no-store"


def check_asset_headers(web, html):
    import re
    asset = re.search(r'src="(/assets/[^"]+\.js)"', html)
    assert asset
    response = request(web, asset[1])
    assert response.status_code == 200
    assert "immutable" in response.headers["Cache-Control"]
    assert response.headers["X-Content-Type-Options"] == "nosniff"


@pytest.mark.parametrize("domain", ["https://site.test", "site.test/path", "127.0.0.1",
                                    "www.site.test", "site.test:443", "bad host.test"])
def test_invalid_domain_is_rejected(domain):
    with pytest.raises(ValueError):
        website.validate_domain(domain)


def test_domain_preserves_secrets_and_is_reused(tmp_path, monkeypatch):
    monkeypatch.setattr(runtime, "CONFIG", tmp_path)
    monkeypatch.setattr(settings, "CONFIG", tmp_path)
    monkeypatch.setattr(website, "CONFIG", tmp_path)
    settings.initialize()
    original = (tmp_path / "production.env").read_text()
    assert runtime.production_domain() == ""
    website.save_domain("nossainflacao.com.br")
    settings.initialize()
    assert runtime.production_domain() == "nossainflacao.com.br"
    assert original in (tmp_path / "production.env").read_text()
    assert (tmp_path / "production.env").stat().st_mode & 0o777 == 0o600
