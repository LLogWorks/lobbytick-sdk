"""Client for the LobbyTick public game-data API.

>>> from lobbytick_gamedata import LobbyTick
>>> api = LobbyTick()
>>> api.get_item("minecraft", "latest", "minecraft:diamond_sword", lang="pt_br").name
'Espada de Diamante'
"""

from ._client import (
    DEFAULT_BASE_URL,
    MAX_PAGE_SIZE,
    HTTPRequest,
    HTTPResponse,
    LobbyTick,
    Transport,
    UrllibTransport,
    backoff_delay,
    encode_segment,
)
from ._errors import (
    APIError,
    BadRequestError,
    ConnectionError,
    LobbyTickError,
    NotFoundError,
    RateLimitError,
    RateLimitInfo,
    TimeoutError,
    parse_retry_after,
)
from ._models import Atlas, Game, Item, ItemPage, Language, LanguageList, Version, VersionSummary
from ._sprite import SpriteRect, icon_url, sprite_rect
from ._version import __version__

__all__ = [
    "APIError",
    "Atlas",
    "BadRequestError",
    "ConnectionError",
    "DEFAULT_BASE_URL",
    "Game",
    "HTTPRequest",
    "HTTPResponse",
    "Item",
    "ItemPage",
    "Language",
    "LanguageList",
    "LobbyTick",
    "LobbyTickError",
    "MAX_PAGE_SIZE",
    "NotFoundError",
    "RateLimitError",
    "RateLimitInfo",
    "SpriteRect",
    "TimeoutError",
    "Transport",
    "UrllibTransport",
    "Version",
    "VersionSummary",
    "__version__",
    "backoff_delay",
    "encode_segment",
    "icon_url",
    "parse_retry_after",
    "sprite_rect",
]
