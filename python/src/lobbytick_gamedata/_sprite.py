"""Helpers for item icons."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from ._models import Atlas, Item

__all__ = ["SpriteRect", "icon_url", "sprite_rect"]


@dataclass(frozen=True)
class SpriteRect:
    """Where an item's icon is inside a version's atlas, in pixels."""

    #: The atlas image URL.
    url: str
    x: int
    y: int
    width: int
    height: int


def icon_url(item: Item) -> Optional[str]:
    """The URL of an item's own icon image, or None for items without one."""
    return item.icon


def sprite_rect(item: Item, atlas: Optional[Atlas]) -> Optional[SpriteRect]:
    """The pixel rectangle of an item's icon in the atlas (``get_version(...).atlas``).

    None when the item has no sprite or the version has no atlas.
    """
    if atlas is None or item.sprite is None:
        return None
    column, row = item.sprite
    return SpriteRect(url=atlas.url, x=column * atlas.cell, y=row * atlas.cell, width=atlas.cell, height=atlas.cell)
