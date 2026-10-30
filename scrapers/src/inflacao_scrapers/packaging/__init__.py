"""SKU contents, independent of advertised price and approximate sold weight."""

from .resolution import AmbiguousPackage, package_from_name, resolve_package

__all__ = ["AmbiguousPackage", "package_from_name", "resolve_package"]
