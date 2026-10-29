"""Resolve configured food leaf categories into verified public browsing paths."""

from __future__ import annotations

from collections.abc import Iterator

from pydantic import BaseModel, ConfigDict, Field, TypeAdapter

from ..transport import Requester
from .config import AngeloniSettings


class Category(BaseModel):
    model_config = ConfigDict(strict=True)
    id: int = Field(gt=0)
    name: str
    has_children: bool = Field(alias="hasChildren")
    children: list[Category]


def catalog_partitions(requester: Requester, settings: AngeloniSettings) -> list[str]:
    categories = fetch_categories(requester, settings)
    requested = set(settings.catalog_category_ids)
    paths = {
        category.id: leaf_path(category, trail)
        for category, trail in walk_categories(categories)
        if category.id in requested
    }
    if set(paths) != requested:
        raise ValueError("Configured catalog categories missing; collection not started")
    return list(paths.values())


def fetch_categories(requester: Requester, settings: AngeloniSettings) -> list[Category]:
    response = requester.request(
        "GET",
        settings.shop_url + "/api/catalog_system/pub/category/tree/5",
        headers=settings.source_headers(),
    )
    if response.status_code != 200:
        raise ValueError(f"Category tree HTTP {response.status_code}; collection not started")
    return TypeAdapter(list[Category]).validate_json(response.content)


def walk_categories(categories: list[Category], trail: tuple = ()) -> Iterator[tuple]:
    for category in categories:
        yield from walk_category(category, trail)


def walk_category(category: Category, trail: tuple) -> Iterator[tuple]:
    path = (*trail, f"category-{len(trail) + 1}/{category.id}")
    yield category, path
    yield from walk_categories(category.children, path)


def leaf_path(category: Category, trail: tuple) -> str:
    if category.has_children or category.children:
        raise ValueError("Configure leaf catalog categories to respect the source pagination limit")
    return "/".join(trail)
