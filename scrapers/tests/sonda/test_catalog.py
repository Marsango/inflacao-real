import re

import httpx
import pytest

from inflacao_scrapers.sonda.catalog import catalog_pages, fetch, listing_pages
from inflacao_scrapers.sonda.pages import category_path, parse_listing
from inflacao_scrapers.transport import Requester

from .test_parsing import PATH, settings, source


def last_page(document):
    document = re.sub(r'<a id="ctl00_conteudo_linkPaginaProxima".*?</a>', "", document, flags=re.S)
    return re.sub(r"\bde \d+", "de 1", document)


def requester_for(response):
    client = httpx.Client(transport=httpx.MockTransport(response))
    return client, Requester(client, interval=0)


def test_repeated_pages_are_incomplete():
    def respond(request):
        document = source()
        if "/0/2/" in request.url.path:
            document = document.replace('value="1"', 'value="2"')
            document = document.replace("/0/2/15", "/0/3/15")
        return httpx.Response(200, text=document)

    client, requester = requester_for(respond)
    with client:
        pages = listing_pages(requester, settings(), None, PATH, [10])
        assert next(pages).observations
        with pytest.raises(ValueError, match="repeated"):
            next(pages)


def test_page_cap_is_not_silent_success():
    client, requester = requester_for(lambda _: httpx.Response(200, text=source()))
    with client:
        pages = listing_pages(requester, settings(), None, PATH, [1])
        next(pages)
        with pytest.raises(ValueError, match="page limit"):
            next(pages)


@pytest.mark.parametrize(
    "document",
    [
        "<html>Blocked</html>",
        '<div class="product-list"></div>',
        source().replace("de 148", "missing"),
    ],
)
def test_malformed_or_missing_pagination_stops_collection(document):
    if document == source():
        document = re.sub(r"\bde \d+", "missing", document)
    with pytest.raises(ValueError):
        parse_listing(document, PATH)


@pytest.mark.parametrize(
    "path",
    [
        "https://evil.example/delivery/categoria/X",
        "/WebService/catalog",
        "/delivery/categoria/X?x=1",
        "/delivery/categoria/X/0/0/15",
    ],
)
def test_only_public_category_routes_are_configurable(path):
    with pytest.raises(ValueError):
        category_path(path)


def test_unknown_or_blocked_source_does_not_become_empty_success():
    client, requester = requester_for(lambda _: httpx.Response(403))
    with client, pytest.raises(ValueError, match="HTTP 403"):
        list(catalog_pages(requester, settings(category_paths=[PATH]), None))


def test_permitted_same_host_category_redirect_is_paced_and_resolved():
    def respond(request):
        if request.url.path == PATH:
            return httpx.Response(302, headers={"Location": "/delivery/categoriafilha/Mercearia"})
        return httpx.Response(200, text=source())

    client, requester = requester_for(respond)
    with client:
        _, path = fetch(requester, settings(), PATH)
    assert path == "/delivery/categoriafilha/Mercearia"


@pytest.mark.parametrize(
    "destination", ["https://evil.example/", "/delivery/login", "/delivery/categoria/X?secret=x"]
)
def test_redirects_to_other_hosts_or_login_are_rejected(destination):
    client, requester = requester_for(
        lambda _: httpx.Response(302, headers={"Location": destination})
    )
    with client, pytest.raises(ValueError):
        fetch(requester, settings(), PATH)


def test_changing_page_count_is_not_silent_success():
    expected_pages = []
    parse_listing(source(), PATH, expected_pages)
    changed = re.sub(r"\bde \d+", f"de {expected_pages[0] + 1}", source())
    with pytest.raises(ValueError, match="page count changed"):
        parse_listing(changed, PATH, expected_pages)
