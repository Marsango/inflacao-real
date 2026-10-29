"""Regional Festval searches share VTEX parsing and the runner's delivery pipeline."""

from ..adapters import Adapter
from ..vtex.fetching import discover
from ..vtex.regional import prepare_region
from ..vtex.search import search_pages
from .config import FestvalSettings

ADAPTER = Adapter("festval", FestvalSettings, search_pages, discover, prepare=prepare_region)
