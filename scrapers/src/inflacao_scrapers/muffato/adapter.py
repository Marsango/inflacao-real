"""Register Muffato collection and diagnostic discovery with the shared runner."""

from ..adapters import Adapter
from ..vtex.fetching import discover
from ..vtex.search import search_pages
from .config import MuffatoSettings

ADAPTER = Adapter("super-muffato", MuffatoSettings, search_pages, discover)
