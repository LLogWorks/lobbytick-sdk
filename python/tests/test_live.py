"""Runs against the real API only when LOBBYTICK_LIVE=1 (a handful of requests)."""

import os

import pytest

from lobbytick_gamedata import LobbyTick, NotFoundError, sprite_rect

pytestmark = pytest.mark.skipif(os.environ.get("LOBBYTICK_LIVE") != "1", reason="set LOBBYTICK_LIVE=1")


def test_live():
    api = LobbyTick(os.environ.get("LOBBYTICK_BASE_URL") or "https://api.lobbytick.com", max_retries=2)
    assert "minecraft" in [g.id for g in api.list_games()]

    v = api.get_version("minecraft")
    assert v.atlas is not None and v.atlas.url.startswith("https://assets.lobbytick.com/")

    langs = api.list_languages("valheim")
    assert langs.default == "en_us"
    assert any(lang.code == "pt_br" for lang in langs.languages)

    page = api.list_items("minecraft", v.version, lang="pt_br", q="espada diamante", limit=5)
    assert page.items[0].id == "minecraft:diamond_sword"

    item = api.get_item("minecraft", v.version, "minecraft:diamond_sword", lang="pt_br")
    assert item.name == "Espada de Diamante"
    rect = sprite_rect(item, v.atlas)
    assert rect is not None and rect.width == v.atlas.cell

    assert sum(1 for _ in api.iter_items("valheim", page_size=5000)) > 500

    with pytest.raises(NotFoundError):
        api.get_game("nope")
