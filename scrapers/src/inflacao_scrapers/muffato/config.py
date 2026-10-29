"""Muffato-specific branch selection and catalog pagination settings."""

from ..config import Identifier
from ..vtex.config import VtexSettings


class MuffatoSettings(VtexSettings):
    shop_id: Identifier = "super-muffato"
    shop_url: str = "https://www.supermuffato.com.br"
