"""Muffato-specific branch selection and catalog pagination settings."""

from ..config import Identifier
from ..vtex.config import VtexSettings


class MuffatoSettings(VtexSettings):
    shop_id: Identifier = "super-muffato"
    shop_url: str = "https://www.supermuffato.com.br"

    def supports_kilogram_prices(self) -> bool:
        return self.sales_channel == "8" and self.seller_id == "1" and self.postal_code is None

    def supports_egg_counts(self) -> bool:
        return self.supports_kilogram_prices()
