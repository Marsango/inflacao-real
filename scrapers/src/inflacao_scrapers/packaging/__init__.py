"""SKU contents, independent of advertised price and approximate sold weight."""

from .resolution import package_from_name, resolve_package

__all__ = ["package_from_name", "resolve_package"]
