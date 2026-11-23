# lobbytick-gamedata

Python client for the [LobbyTick](https://lobbytick.com) public game-data API: items of
Minecraft, Valheim and Project Zomboid per version, their names in every language and
their icons. Python 3.9+, **no dependencies** (stdlib `urllib`), fully typed (`py.typed`).

```sh
pip install lobbytick-gamedata
```

## Quick start

```python
from lobbytick_gamedata import LobbyTick, NotFoundError, RateLimitError, sprite_rect

api = LobbyTick()  # or LobbyTick(max_retries=3, timeout=10)

games = api.list_games()
version = api.get_version("minecraft")  # "latest" by default
langs = api.list_languages("minecraft", version.version)

page = api.list_items("minecraft", lang="pt_br", q="espada", limit=20)
print(page.total, [i.name for i in page.items])

for item in api.iter_items("zomboid", kind="flat"):  # every page, fetched as needed
    ...

sword = api.get_item("minecraft", "latest", "minecraft:diamond_sword", lang="ja_jp")
rect = sprite_rect(sword, version.atlas)  # SpriteRect(url, x, y, width, height) or None

try:
    api.get_game("nope")
except NotFoundError as e:
    print(e.status, e.message)  # 404 unknown game
except RateLimitError as e:
    print(f"retry in {e.retry_after}s", e.rate_limit)
```

## Options

`LobbyTick(base_url="https://api.lobbytick.com", *, timeout=30.0, transport=None,
max_retries=0, max_retry_delay=60.0, user_agent=None, headers=None)`

- `max_retries`: retries on 429/502/503/504, sleeping for `Retry-After` (else
  `RateLimit-Reset`, else 1 s, 2 s, 4 s … 30 s). Never sleeps longer than
  `max_retry_delay`; raises instead.
- `transport`: any callable `HTTPRequest -> HTTPResponse` (for tests, proxies, or another
  HTTP library). The default is `UrllibTransport`.
- `api.last_rate_limit` holds the `RateLimit-*` headers of the last successful answer.

Errors: `LobbyTickError` ← `APIError` (`status`, `message`, `body`, `headers`,
`rate_limit`) ← `BadRequestError` / `NotFoundError` / `RateLimitError` (`retry_after`);
plus `ConnectionError` and `TimeoutError` (this package's, not the builtins).

Models are frozen dataclasses with the API's field names; `.raw` keeps the original dict.

## Data license

The code is MIT. Item names and images belong to Mojang/Microsoft, Iron Gate and The Indie
Stone and are not covered by the MIT license.
