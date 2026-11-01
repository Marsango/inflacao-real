"""Small HTML tree for source-specific selectors, without browser dependencies."""

from dataclasses import dataclass, field
from html.parser import HTMLParser

VOID = {"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "wbr"}


@dataclass
class Node:
    tag: str
    attributes: dict = field(default_factory=dict)
    children: list = field(default_factory=list)


class Document(HTMLParser):
    def __init__(self, source):
        super().__init__(convert_charrefs=True)
        self.root = Node("document")
        self.stack = [self.root]
        self.feed(source)
        self.close()

    def handle_starttag(self, tag, attributes):
        node = Node(tag, dict(attributes))
        self.stack[-1].children.append(node)
        if tag not in VOID:
            self.stack.append(node)

    def handle_startendtag(self, tag, attributes):
        self.handle_starttag(tag, attributes)
        if tag not in VOID:
            self.handle_endtag(tag)

    def handle_endtag(self, tag):
        index = next(
            (i for i in range(len(self.stack) - 1, 0, -1) if self.stack[i].tag == tag), None
        )
        if index is not None:
            del self.stack[index:]

    def handle_data(self, data):
        if self.stack[-1].tag not in {"script", "style"}:
            self.stack[-1].children.append(data)


def nodes(root):
    if "related" in root.attributes.get("class", "").split():
        return
    yield root
    for child in [child for child in root.children if isinstance(child, Node)]:
        yield from nodes(child)


def select(root, **attributes):
    return [
        node
        for node in nodes(root)
        if all(node.attributes.get(key) == value for key, value in attributes.items())
    ]


def class_nodes(root, name):
    return [node for node in nodes(root) if name in node.attributes.get("class", "").split()]


def suffix_nodes(root, suffix):
    return [node for node in nodes(root) if node.attributes.get("id", "").endswith(suffix)]


def text(root):
    fragments = [child_text(child) for child in root.children]
    return " ".join(" ".join(fragments).split())


def child_text(child):
    if isinstance(child, str):
        return child
    return text(child)


def unique_text(matches, label, required=True):
    if not matches and not required:
        return None
    if len(matches) != 1 or not text(matches[0]):
        raise ValueError(f"Missing or ambiguous Sonda {label}")
    return text(matches[0])
