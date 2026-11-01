"""Validate listing structure and pagination; never treat a block as an empty catalog."""

import re
from urllib.parse import unquote

from .html import Document, class_nodes, select, suffix_nodes, text, unique_text


def category_path(path):
    if not re.fullmatch(
        r"/delivery/(?:categoria|categoriafilha)/[\w%.-]+(?:/0/[1-9]\d*/(?:15|16|21|30))?", path
    ):
        raise ValueError("Invalid Sonda category path")
    decoded = unquote(path)
    if not re.fullmatch(
        r"/delivery/(?:categoria|categoriafilha)/[\w.-]+(?:/0/[1-9]\d*/(?:15|16|21|30))?", decoded
    ) or decoded.split("/")[3] in {".", ".."}:
        raise ValueError("Invalid Sonda category path")
    return path


def parse_listing(source, path, expected_pages=None):
    root = Document(source).root
    listings = class_nodes(root, "product-list")
    if len(listings) != 1:
        raise ValueError("Missing Sonda product listing; collection incomplete")
    cards = suffix_nodes(listings[0], "_divGeral")
    if not cards:
        raise ValueError("Empty Sonda listing requires verification; collection incomplete")
    next_path = pagination(root, path, expected_pages)
    return cards, next_path


def pagination(root, path, expected_pages=None):
    panel = select(root, id="ctl00_conteudo_pnlPaginacao")
    if len(panel) != 1:
        raise ValueError("Missing Sonda pagination")
    current = select(panel[0], id="ctl00_conteudo_txtPagina")
    total = re.search(r"\bde\s+(\d+)\s*$", text(panel[0]))
    page = page_number(path)
    if len(current) != 1 or current[0].attributes.get("value") != str(page) or not total:
        raise ValueError("Inconsistent Sonda pagination")
    verify_page_count(int(total[1]), expected_pages)
    return next_page(panel[0], path, page, int(total[1]))


def page_number(path):
    parts = path.split("/")
    if len(parts) == 4:
        return 1
    return int(parts[-2])


def next_page(panel, path, page, total):
    links = suffix_nodes(panel, "_linkPaginaProxima")
    if total < page:
        raise ValueError("Invalid Sonda page count")
    if total == page and not links:
        return None
    size = path.split("/")[-1] if "/0/" in path else page_size(links)
    expected = path.split("/0/")[0] + f"/0/{page + 1}/{size}"
    if (
        total == page
        or len(links) != 1
        or unquote(links[0].attributes.get("href", "")) != unquote(expected)
    ):
        raise ValueError("Invalid Sonda next page; collection incomplete")
    return category_path(links[0].attributes["href"])


def product_identity(card):
    identity = unique_text(class_nodes(card, "js-sku"), "SKU")
    if not re.fullmatch(r"[1-9]\d*", identity):
        raise ValueError("Invalid Sonda SKU")
    return identity


def category_paths(source):
    root = Document(source).root
    links = [node for node in suffix_menu(root)]
    paths = list(dict.fromkeys(category_path(node.attributes.get("href", "")) for node in links))
    if not paths:
        raise ValueError("Missing Sonda category navigation")
    return paths


def suffix_menu(root):
    return [
        node
        for node in select_tags(root)
        if re.fullmatch(
            r"ctl00_rptMenuDestaque(?:Mobile)?_ctl\d+_linkMenu", node.attributes.get("id", "")
        )
    ]


def select_tags(root):
    from .html import nodes

    return [node for node in nodes(root) if node.tag == "a"]


def page_size(links):
    if len(links) != 1:
        raise ValueError("Missing Sonda next-page link")
    path = category_path(links[0].attributes.get("href", ""))
    return path.split("/")[-1]


def verify_page_count(total, expected_pages):
    if expected_pages is None:
        return
    if expected_pages and expected_pages[0] != total:
        raise ValueError("Sonda page count changed during collection; collection incomplete")
    if not expected_pages:
        expected_pages.append(total)
