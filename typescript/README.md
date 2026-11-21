# @lobbytick/gamedata

TypeScript/JavaScript client for the [LobbyTick](https://lobbytick.com) public game-data
API: items of Minecraft, Valheim and Project Zomboid per version, their names in every
language and their icons. Zero runtime dependencies; ESM + CommonJS; Node 18+, Deno, Bun
and browsers (uses the global `fetch`).

> The npm scope `@lobbytick` must be created on npmjs.com before the first publish (see
> the repository README). Until then, install from the repository.

```sh
npm install @lobbytick/gamedata
```

## Quick start

```ts
import { LobbyTick, NotFoundError, RateLimitError, spriteRect } from "@lobbytick/gamedata";

const api = new LobbyTick(); // or new LobbyTick({ maxRetries: 3, timeout: 10_000 })

const games = await api.listGames();
const version = await api.getVersion("minecraft"); // "latest" by default
const { languages } = await api.listLanguages("minecraft", version.version);

const page = await api.listItems("minecraft", "latest", { lang: "pt_br", q: "espada", limit: 20 });
console.log(page.total, page.items.map((i) => i.name));

for await (const item of api.iterItems("zomboid", "latest", { kind: "flat" })) {
  // every page, fetched as needed
}

const sword = await api.getItem("minecraft", "latest", "minecraft:diamond_sword", { lang: "ja_jp" });
const rect = spriteRect(sword, version.atlas); // { url, x, y, width, height } or null
// CSS: background: url(rect.url) -{x}px -{y}px; width: {width}px; height: {height}px

try {
  await api.getGame("nope");
} catch (e) {
  if (e instanceof NotFoundError) console.log(e.status, e.message); // 404 "unknown game"
  if (e instanceof RateLimitError) console.log(`retry in ${e.retryAfter}s`, e.rateLimit);
}
```

## Options

| Option          | Default                              | Notes                                                       |
| --------------- | ------------------------------------ | ----------------------------------------------------------- |
| `baseUrl`       | `https://api.lobbytick.com`          |                                                             |
| `timeout`       | `30000` ms                           | per attempt; `0` disables                                   |
| `fetch`         | `globalThis.fetch`                   | any fetch-compatible function                               |
| `maxRetries`    | `0`                                  | retries on 429/502/503/504                                  |
| `maxRetryDelay` | `60` s                               | never sleeps longer; throws instead                         |
| `userAgent`     | `lobbytick-sdk-typescript/0.1.0`     | not sent from browsers (would force a CORS preflight)       |
| `headers`       | `{}`                                 | extra headers                                               |

Every method also accepts `signal` (an `AbortSignal`).

Errors: `LobbyTickError` ← `APIError` (`status`, `message`, `body`, `headers`, `rateLimit`)
← `BadRequestError` / `NotFoundError` / `RateLimitError` (`retryAfter`); plus
`ConnectionError` and `TimeoutError`.

Models keep the API's field names (`max_stack`, `world_version`, ...).

## Data license

The code is MIT. Item names and images belong to Mojang/Microsoft, Iron Gate and The Indie
Stone and are not covered by the MIT license.
