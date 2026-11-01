"""Public MASP-address CEP; prices retain the resolved online delivery region."""

from pydantic import Field

from ..config import Identifier
from ..vtex.config import VtexSettings


class MamboSettings(VtexSettings):
    adapter: str = "mambo"
    shop_id: Identifier = "mambo"
    shop_url: str = "https://www.mambo.com.br"
    postal_code: str = Field(default="01310200", pattern=r"^[0-9]{8}$")
    regional_seller_id: str = Field(default="mambodelivery215", min_length=1)
    city: str = "São Paulo"
    state: str = "SP"
    store_id: str = "seller:mambodelivery215"

    def supports_kilogram_prices(self) -> bool:
        return (
            self.shop_url == "https://www.mambo.com.br"
            and self.sales_channel == "1"
            and self.seller_id == "1"
            and self.postal_code == "01310200"
            and self.regional_seller_id == "mambodelivery215"
            and self.region_id is not None
            and self.city == "São Paulo"
            and self.state == "SP"
        )
