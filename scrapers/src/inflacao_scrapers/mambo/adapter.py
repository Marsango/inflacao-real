"""Mambo catalog requests use the existing regional VTEX delivery pipeline."""

from ..adapters import Adapter
from ..vtex.fetching import discover
from ..vtex.regional import prepare_region
from ..vtex.search import search_pages
from .config import MamboSettings

ADAPTER = Adapter("mambo", MamboSettings, search_pages, discover, prepare=prepare_region)
