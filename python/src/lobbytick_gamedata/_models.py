"""Models of the LobbyTick game-data API responses.

Field names follow the API (snake_case). Unknown fields the API may add later
are ignored, so new server versions do not break old clients; ``raw`` keeps the
original dict for anything not modelled yet.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Mapping, Optional, Tuple

__all__ = [
    "Atlas",
    "Game",
    "Item",
    "ItemPage",
    "Language",
    "LanguageList",
    "Version",
    "VersionSummary",
]

#: How an item's icon was made: "flat", "block", "special" (no icon yet) or "empty".
ItemKind = str


@dataclass(frozen=True)
class VersionSummary:
    version: str
    protocol: int
    items: int
    languages: int

    @classmethod
    def from_dict(cls, d: Mapping[str, Any]) -> "VersionSummary":
        return cls(
            version=d["version"],
            protocol=d.get("protocol", 0),
            items=d.get("items", 0),
            languages=d.get("languages", 0),
        )


@dataclass(frozen=True)
class Game:
    id: str
    name: str
    #: Newest first. Empty for a game LobbyTick has no data for yet.
    versions: List[VersionSummary]
    raw: Dict[str, Any] = field(default_factory=dict, repr=False, compare=False)

    @classmethod
    def from_dict(cls, d: Mapping[str, Any]) -> "Game":
        return cls(
            id=d["id"],
            name=d.get("name", ""),
            versions=[VersionSummary.from_dict(v) for v in d.get("versions") or []],
            raw=dict(d),
        )


@dataclass(frozen=True)
class Atlas:
    """Every icon of a version in one image: a grid of square cells."""

    url: str
    columns: int
    rows: int
    #: Side of each cell, in pixels.
    cell: int

    @classmethod
    def from_dict(cls, d: Mapping[str, Any]) -> "Atlas":
        return cls(url=d["url"], columns=d["columns"], rows=d["rows"], cell=d["cell"])


@dataclass(frozen=True)
class Version:
    version: str
    protocol: int
    #: 0 when the game has no such thing.
    world_version: int
    items: int
    languages: int
    #: Side of each icon, in pixels.
    icon_size: int
    atlas: Optional[Atlas]
    raw: Dict[str, Any] = field(default_factory=dict, repr=False, compare=False)

    @classmethod
    def from_dict(cls, d: Mapping[str, Any]) -> "Version":
        return cls(
            version=d["version"],
            protocol=d.get("protocol", 0),
            world_version=d.get("world_version", 0),
            items=d.get("items", 0),
            languages=d.get("languages", 0),
            icon_size=d.get("icon_size", 0),
            atlas=Atlas.from_dict(d["atlas"]) if d.get("atlas") else None,
            raw=dict(d),
        )


@dataclass(frozen=True)
class Language:
    #: The game's own code, e.g. "pt_br".
    code: str
    name: str
    region: str
    #: How many items it names; the rest fall back to English.
    items: int

    @classmethod
    def from_dict(cls, d: Mapping[str, Any]) -> "Language":
        return cls(code=d["code"], name=d.get("name", ""), region=d.get("region", ""), items=d.get("items", 0))


@dataclass(frozen=True)
class LanguageList:
    version: str
    #: The language used when no ``lang`` is given ("en_us").
    default: str
    languages: List[Language]

    @classmethod
    def from_dict(cls, d: Mapping[str, Any]) -> "LanguageList":
        return cls(
            version=d["version"],
            default=d.get("default", "en_us"),
            languages=[Language.from_dict(x) for x in d.get("languages") or []],
        )


@dataclass(frozen=True)
class Item:
    #: What the game's commands take, e.g. "minecraft:diamond_sword".
    id: str
    #: Name in the requested language (falls back to English, then to the id).
    name: str
    kind: ItemKind
    #: URL of the item's own image, or None for items the game draws by code.
    icon: Optional[str]
    #: (column, row) of the icon in the version's atlas, or None.
    sprite: Optional[Tuple[int, int]]
    #: How many fit in one slot; None when not known.
    max_stack: Optional[int] = None
    #: Highest quality the item is made at (games that have quality, e.g. Valheim).
    max_quality: Optional[int] = None
    #: What the game itself says the item is ("material", "helmet"...), when known.
    type: Optional[str] = None
    raw: Dict[str, Any] = field(default_factory=dict, repr=False, compare=False)

    @classmethod
    def from_dict(cls, d: Mapping[str, Any]) -> "Item":
        sprite = d.get("sprite")
        return cls(
            id=d["id"],
            name=d.get("name", d["id"]),
            kind=d.get("kind", ""),
            icon=d.get("icon"),
            sprite=(int(sprite[0]), int(sprite[1])) if sprite else None,
            max_stack=d.get("max_stack"),
            max_quality=d.get("max_quality"),
            type=d.get("type"),
            raw=dict(d),
        )


@dataclass(frozen=True)
class ItemPage:
    version: str
    lang: str
    #: How many items match the query in total.
    total: int
    offset: int
    limit: int
    items: List[Item]

    @classmethod
    def from_dict(cls, d: Mapping[str, Any]) -> "ItemPage":
        return cls(
            version=d["version"],
            lang=d.get("lang", "en_us"),
            total=d.get("total", 0),
            offset=d.get("offset", 0),
            limit=d.get("limit", 0),
            items=[Item.from_dict(x) for x in d.get("items") or []],
        )
