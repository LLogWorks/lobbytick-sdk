# LobbyTick SDKs

Official clients for the [LobbyTick](https://lobbytick.com) public **game-data API**
(`https://api.lobbytick.com`): the items of Minecraft, Valheim and Project Zomboid per
game version, their names in every language the game ships, and their icons.

The API is read-only, needs no key or login, answers JSON and allows **120 requests per
minute per visitor**. Images are served from `https://assets.lobbytick.com` (one PNG per
item, plus one atlas per version).

| Language   | Folder                        | Package                         | Install                                   |
| ---------- | ----------------------------- | ------------------------------- | ----------------------------------------- |
| TypeScript | [`typescript/`](typescript/)  | `@lobbytick/gamedata` (npm)     | `npm install @lobbytick/gamedata`         |
| Python     | [`python/`](python/)          | `lobbytick-gamedata` (PyPI)     | `pip install lobbytick-gamedata`          |
| Go         | [`go/`](go/)                  | `github.com/LLogWorks/lobbytick-sdk/go` | `go get github.com/LLogWorks/lobbytick-sdk/go` |

All three are hand-written, small and have **no runtime dependencies** (TypeScript uses the
global `fetch`, Python the stdlib `urllib`, Go `net/http`). They expose the same surface:

| Operation                     | TypeScript                 | Python                       | Go                                |
| ----------------------------- | -------------------------- | ---------------------------- | --------------------------------- |
| `GET /v1/games`               | `listGames()`              | `list_games()`               | `ListGames(ctx)`                  |
| `GET /v1/games/{game}`        | `getGame(game)`            | `get_game(game)`             | `GetGame(ctx, game)`              |
| `GET …/versions/{version}`    | `getVersion(game, v)`      | `get_version(game, v)`       | `GetVersion(ctx, game, v)`        |
| `GET …/languages`             | `listLanguages(game, v)`   | `list_languages(game, v)`    | `ListLanguages(ctx, game, v)`     |
| `GET …/items`                 | `listItems(game, v, opts)` | `list_items(game, v, ...)`   | `ListItems(ctx, game, v, params)` |
| every item, all pages         | `iterItems()` (async gen.) | `iter_items()` (generator)   | `IterItems()` (`iter.Seq2`)       |
| `GET …/items/{id}`            | `getItem(game, v, id)`     | `get_item(game, v, id)`      | `GetItem(ctx, game, v, id, lang)` |
| icon position in the atlas    | `spriteRect(item, atlas)`  | `sprite_rect(item, atlas)`   | `item.SpriteRect(atlas)`          |

`version` defaults to `"latest"` (the newest version LobbyTick has data for).

## Quick start

```ts
import { LobbyTick } from "@lobbytick/gamedata";
const api = new LobbyTick();
const sword = await api.getItem("minecraft", "latest", "minecraft:diamond_sword", { lang: "pt_br" });
console.log(sword.name, sword.icon); // Espada de Diamante https://assets.lobbytick.com/...
```

```python
from lobbytick_gamedata import LobbyTick
api = LobbyTick()
for item in api.iter_items("valheim", lang="pt_br", q="espada"):
    print(item.id, item.name)
```

```go
c := lobbytick.New(nil)
item, err := c.GetItem(ctx, "zomboid", "latest", "Base.Axe", "")
```

## Behaviour shared by all three

- **Errors** — any non-2xx answer becomes a typed error carrying the HTTP status and the
  API's own `error` message (`{"error": "unknown game"}`): `NotFoundError` (404),
  `BadRequestError` (400), `RateLimitError` (429), or the base `APIError`. In Go it is one
  `*APIError`, testable with `errors.Is(err, lobbytick.ErrNotFound)` / `ErrBadRequest` /
  `ErrRateLimited`. Network failures and timeouts are separate errors.
- **Rate limit (429)** — the API sends `Retry-After` (seconds) plus `RateLimit-Limit`,
  `RateLimit-Remaining` and `RateLimit-Reset` on every answer. By default the client does
  **not** retry: it raises the 429 error with `retryAfter` / `retry_after` / `RetryAfter`
  set. With `maxRetries` / `max_retries` / `MaxRetries` > 0, it sleeps for `Retry-After`
  (falling back to `RateLimit-Reset`, then to a 1 s, 2 s, 4 s … 30 s backoff) and tries
  again, up to that many times. It never sleeps longer than `maxRetryDelay` (default 60 s):
  if the API asks for more, the error is raised. 502/503/504 are retried the same way
  (Retry-After if sent, otherwise the backoff).
- **Pagination** — `listItems` returns one page (`total`, `offset`, `limit`, `items`;
  `limit` 1..5000, server default 50). The iterator walks every page (default page size
  1000) and, when asked for `latest`, pins the concrete version from the first page so a
  deploy in the middle cannot mix two catalogs.
- **Icons** — `item.icon` is the URL of the item's own PNG (null for items the game draws
  by code, kind `special`). `item.sprite` is `[column, row]` in the version's atlas
  (`getVersion().atlas`: `url`, `columns`, `rows`, `cell`); the sprite helper turns it
  into a pixel rectangle `{url, x, y, width, height}`.
- **Configuration** — base URL (default `https://api.lobbytick.com`), timeout (30 s),
  custom `fetch` / transport / `*http.Client`, and a `User-Agent` of
  `lobbytick-sdk-<lang>/<version>` (not sent from browsers, where it would force a CORS
  preflight the API does not answer).
- **Item ids keep `:` unescaped** in the path (`/items/minecraft:diamond_sword`): the API
  does not find `minecraft%3Adiamond_sword`.

## Data license

The SDK source code is MIT-licensed (see [LICENSE](LICENSE)). **The data the API returns
is not**: item names, translations, icons and atlas images belong to the makers of each
game — Mojang Studios / Microsoft (Minecraft), Iron Gate AB (Valheim) and The Indie Stone
(Project Zomboid). Use them under those companies' own terms (e.g. the
[Minecraft Usage Guidelines](https://www.minecraft.net/usage-guidelines)). LobbyTick is not
affiliated with or endorsed by any of them.

## Development

```sh
cd typescript && npm ci && npm test && npm run build
cd python && python3 -m venv .venv && .venv/bin/pip install -e '.[dev]' && .venv/bin/pytest
cd go && go vet ./... && go test ./...
```

Each suite has one live test against the real API, skipped unless `LOBBYTICK_LIVE=1`
(optionally `LOBBYTICK_BASE_URL=...`). It makes about seven requests.

## Releasing

Versions are independent per language and released by tag:

| Tag               | What happens                                                                  |
| ----------------- | ----------------------------------------------------------------------------- |
| `typescript/vX.Y.Z` | `release-typescript.yml` tests and publishes to npm with provenance        |
| `python/vX.Y.Z`   | `release-python.yml` tests, builds and publishes to PyPI (trusted publishing) |
| `go/vX.Y.Z`       | nothing to publish: the tag is the release (`release-go.yml` tests and warms proxy.golang.org) |

Before tagging, bump the version in the package (`typescript/package.json` +
`typescript/src/version.ts`, `python/src/lobbytick_gamedata/_version.py`, `SDKVersion` in
`go/client.go`) and add a `CHANGELOG.md` entry. The workflows refuse a tag that does not
match.

### One-time setup for the maintainer

1. **GitHub repository** — create the public repo `LLogWorks/lobbytick-sdk` and push
   `main`. In *Settings → Environments*, create two environments, `npm` and `pypi`
   (optionally with required reviewers and a "tags only" deployment rule:
   `typescript/v*` and `python/v*`). Protect `main` and require the CI checks.
2. **npm** — create the `lobbytick` organization on npmjs.com (the scope `@lobbytick`
   must exist before anything can be published under it; otherwise rename the package in
   `typescript/package.json`). The first publish needs a token: create a granular access
   token with publish rights on the scope and save it as the secret `NPM_TOKEN` in the
   `npm` environment, then push `typescript/v0.1.0`. Afterwards, switch to tokenless
   publishing: on npmjs.com, package `@lobbytick/gamedata` → *Settings → Trusted
   Publisher → GitHub Actions*: owner `LLogWorks`, repository `lobbytick-sdk`, workflow
   `release-typescript.yml`, environment `npm`; then delete the `NPM_TOKEN` secret.
   Provenance works either way (the workflow has `id-token: write`).
3. **PyPI** — on pypi.org, *Your account → Publishing → Add a new pending publisher*
   (works before the project exists): project name `lobbytick-gamedata`, owner
   `LLogWorks`, repository `lobbytick-sdk`, workflow `release-python.yml`, environment
   `pypi`. No token or secret is needed. Then push `python/v0.1.0`.
4. **Go** — nothing to register. Push `go/v0.1.0`; pkg.go.dev picks it up after the first
   `go get` (the release workflow triggers that).

---

## Em português

SDKs oficiais da **API pública de dados de jogos** do LobbyTick
(`https://api.lobbytick.com`): os itens de Minecraft, Valheim e Project Zomboid por
versão, com nomes em todos os idiomas do jogo e ícones. A API é só leitura, não precisa
de chave e permite **120 requisições por minuto por visitante**.

- TypeScript: `npm install @lobbytick/gamedata` — Python: `pip install lobbytick-gamedata` —
  Go: `go get github.com/LLogWorks/lobbytick-sdk/go`.
- Erros são tipados e trazem o status HTTP e a mensagem da API. No 429 o erro traz
  `retryAfter` (do cabeçalho `Retry-After`); com `maxRetries` o cliente espera e tenta de
  novo sozinho, até o limite configurado.
- O iterador percorre todas as páginas de itens; o helper de sprite devolve o retângulo
  do ícone dentro do atlas.
- O código dos SDKs é MIT. **Os dados** (nomes dos itens e imagens) pertencem às
  empresas de cada jogo — Mojang/Microsoft, Iron Gate e The Indie Stone — e **não** estão
  cobertos pela licença MIT.
