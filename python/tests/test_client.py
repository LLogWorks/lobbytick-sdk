from __future__ import annotations

import json
from typing import Callable, Dict, List, Optional
from urllib.parse import parse_qs, urlsplit

import pytest

from lobbytick_gamedata import (
    APIError,
    Atlas,
    BadRequestError,
    ConnectionError,
    HTTPRequest,
    HTTPResponse,
    Item,
    LobbyTick,
    NotFoundError,
    RateLimitError,
    SpriteRect,
    __version__,
    backoff_delay,
    encode_segment,
    icon_url,
    parse_retry_after,
    sprite_rect,
)


def respond(body, status: int = 200, headers: Optional[Dict[str, str]] = None) -> HTTPResponse:
    raw = body if isinstance(body, bytes) else json.dumps(body).encode()
    return HTTPResponse(status, {"content-type": "application/json", **(headers or {})}, raw)


class Mock:
    def __init__(self, handler: Callable[[HTTPRequest], HTTPResponse]) -> None:
        self.handler = handler
        self.calls: List[HTTPRequest] = []

    def __call__(self, req: HTTPRequest) -> HTTPResponse:
        self.calls.append(req)
        return self.handler(req)

    def paths(self) -> List[str]:
        out = []
        for c in self.calls:
            u = urlsplit(c.url)
            out.append(u.path + ("?" + u.query if u.query else ""))
        return out


SWORD = {
    "id": "minecraft:diamond_sword",
    "name": "Diamond Sword",
    "kind": "flat",
    "icon": "https://assets.lobbytick.com/games/minecraft/26.3/items/diamond_sword.png",
    "sprite": [3, 2],
    "max_stack": 1,
}


def test_list_games_and_user_agent():
    m = Mock(lambda r: respond({"games": [{"id": "minecraft", "name": "Minecraft", "versions": [
        {"version": "26.3", "protocol": 777, "items": 1658, "languages": 145}]}]},
        headers={"ratelimit-limit": "120", "ratelimit-remaining": "119", "ratelimit-reset": "60"}))
    api = LobbyTick(transport=m)
    games = api.list_games()
    assert games[0].id == "minecraft"
    assert games[0].versions[0].items == 1658
    assert m.calls[0].url == "https://api.lobbytick.com/v1/games"
    assert m.calls[0].headers["User-Agent"] == f"lobbytick-sdk-python/{__version__}"
    assert m.calls[0].timeout == 30.0
    assert api.last_rate_limit is not None and api.last_rate_limit.remaining == 119


def test_routes():
    m = Mock(lambda r: respond({"version": "1", "id": "x", "items": [], "languages": []}))
    api = LobbyTick("http://localhost:8080/", transport=m, timeout=5)
    api.get_game("valheim")
    api.get_version("valheim")
    api.list_languages("valheim", "1.0.21")
    api.list_items("minecraft", lang="pt_br", q="espada diamante", kind="flat", limit=10, offset=20)
    api.get_item("minecraft", "26.3", "minecraft:diamond_sword", lang="ja_jp")
    assert m.paths() == [
        "/v1/games/valheim",
        "/v1/games/valheim/versions/latest",
        "/v1/games/valheim/versions/1.0.21/languages",
        "/v1/games/minecraft/versions/latest/items?lang=pt_br&q=espada+diamante&kind=flat&limit=10&offset=20",
        "/v1/games/minecraft/versions/26.3/items/minecraft:diamond_sword?lang=ja_jp",
    ]
    assert m.calls[0].url.startswith("http://localhost:8080/")


def test_encode_segment():
    assert encode_segment("minecraft:oak_log") == "minecraft:oak_log"
    assert encode_segment("Base.3030Box") == "Base.3030Box"
    assert encode_segment("a/b c") == "a%2Fb%20c"


def test_models():
    m = Mock(lambda r: respond({
        "atlas": {"url": "https://a/atlas.png", "columns": 40, "rows": 40, "cell": 64},
        "icon_size": 64, "items": 1658, "languages": 145, "protocol": 777, "version": "26.3",
        "world_version": 5023, "future_field": True,
    }))
    v = LobbyTick(transport=m).get_version("minecraft")
    assert v.atlas == Atlas("https://a/atlas.png", 40, 40, 64)
    assert v.world_version == 5023
    assert v.raw["future_field"] is True

    chest = Item.from_dict({"id": "minecraft:chest", "name": "Chest", "kind": "special", "icon": None, "sprite": None})
    assert chest.sprite is None and chest.max_stack is None


def test_typed_errors():
    def handler(r: HTTPRequest) -> HTTPResponse:
        if r.url.endswith("/nope"):
            return respond({"error": "unknown game"}, 404)
        return respond({"error": "unknown lang: see the version's /languages"}, 400)

    api = LobbyTick(transport=Mock(handler))
    with pytest.raises(NotFoundError) as e:
        api.get_game("nope")
    assert e.value.status == 404 and e.value.message == "unknown game"
    assert isinstance(e.value, APIError)
    with pytest.raises(BadRequestError, match="unknown lang"):
        api.list_items("minecraft", lang="xx_xx")


def test_non_json_error():
    api = LobbyTick(transport=Mock(lambda r: HTTPResponse(502, {}, b"<html>bad gateway</html>")))
    with pytest.raises(APIError) as e:
        api.list_games()
    assert e.value.status == 502 and e.value.message == "HTTP 502"
    assert e.value.body == "<html>bad gateway</html>"


def test_connection_error_from_default_transport():
    # Port 9 on localhost: nothing listens there.
    api = LobbyTick("http://127.0.0.1:9", timeout=2)
    with pytest.raises(ConnectionError):
        api.list_games()


LIMITED = {"retry-after": "7", "ratelimit-limit": "120", "ratelimit-remaining": "0", "ratelimit-reset": "7"}


def test_rate_limit_raises_by_default():
    m = Mock(lambda r: respond({"error": "too many requests: wait and try again"}, 429, LIMITED))
    with pytest.raises(RateLimitError) as e:
        LobbyTick(transport=m).list_games()
    assert e.value.retry_after == 7
    assert e.value.rate_limit.limit == 120 and e.value.rate_limit.remaining == 0
    assert len(m.calls) == 1


def test_rate_limit_retries_with_retry_after():
    sleeps: List[float] = []
    n = {"i": 0}

    def handler(r):
        n["i"] += 1
        return respond({"error": "too many requests"}, 429, LIMITED) if n["i"] == 1 else respond({"games": []})

    m = Mock(handler)
    assert LobbyTick(transport=m, max_retries=2, sleep=sleeps.append).list_games() == []
    assert sleeps == [7]
    assert len(m.calls) == 2


def test_backoff_without_hints_and_cap():
    sleeps: List[float] = []
    m = Mock(lambda r: respond({"error": "busy"}, 503))
    with pytest.raises(APIError):
        LobbyTick(transport=m, max_retries=3, sleep=sleeps.append).list_games()
    assert sleeps == [1, 2, 4]
    assert len(m.calls) == 4

    m = Mock(lambda r: respond({"error": "too many requests"}, 429, {"retry-after": "120"}))
    with pytest.raises(RateLimitError):
        LobbyTick(transport=m, max_retries=3, max_retry_delay=60, sleep=sleeps.append).list_games()
    assert len(m.calls) == 1


def test_parse_retry_after():
    assert parse_retry_after("5") == 5
    assert parse_retry_after(None) is None
    assert parse_retry_after("soon") is None
    from email.utils import parsedate_to_datetime
    now = parsedate_to_datetime("Sat, 21 Nov 2026 12:00:00 GMT").timestamp()
    assert parse_retry_after("Sat, 21 Nov 2026 12:00:10 GMT", now=now) == 10
    assert [backoff_delay(i) for i in range(7)] == [1, 2, 4, 8, 16, 30, 30]


def test_iter_items_pins_version():
    all_items = [dict(SWORD, id=f"x:{i}") for i in range(5)]

    def handler(r):
        q = parse_qs(urlsplit(r.url).query)
        limit, offset = int(q["limit"][0]), int(q["offset"][0])
        return respond({"version": "26.3", "lang": "en_us", "total": 5, "offset": offset, "limit": limit,
                        "items": all_items[offset:offset + limit]})

    m = Mock(handler)
    ids = [it.id for it in LobbyTick(transport=m).iter_items("minecraft", kind="flat", page_size=2)]
    assert ids == [f"x:{i}" for i in range(5)]
    assert len(m.calls) == 3
    assert "/versions/latest/" in m.calls[0].url
    assert "/versions/26.3/" in m.calls[1].url
    assert "kind=flat" in m.calls[2].url and "offset=4" in m.calls[2].url


def test_iter_items_stops_on_empty_page_and_validates():
    m = Mock(lambda r: respond({"version": "1", "lang": "en_us", "total": 10, "offset": 0, "limit": 5, "items": []}))
    assert list(LobbyTick(transport=m).iter_items("valheim")) == []
    assert len(m.calls) == 1
    with pytest.raises(ValueError):
        list(LobbyTick(transport=m).iter_items("valheim", page_size=5001))


def test_sprite_helpers():
    atlas = Atlas("https://a/atlas.png", 40, 40, 64)
    sword = Item.from_dict(SWORD)
    assert sprite_rect(sword, atlas) == SpriteRect("https://a/atlas.png", 192, 128, 64, 64)
    assert icon_url(sword) == SWORD["icon"]
    chest = Item.from_dict({"id": "c", "name": "C", "kind": "special", "icon": None, "sprite": None})
    assert sprite_rect(chest, atlas) is None
    assert sprite_rect(sword, None) is None
    assert icon_url(chest) is None
