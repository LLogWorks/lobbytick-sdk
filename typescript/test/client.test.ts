import { describe, expect, it, vi } from "vitest";
import {
  APIError,
  BadRequestError,
  ConnectionError,
  LobbyTick,
  NotFoundError,
  RateLimitError,
  TimeoutError,
  VERSION,
  backoffDelay,
  encodeSegment,
  iconUrl,
  parseRetryAfter,
  spriteRect,
  type Item,
} from "../src/index.js";

type Handler = (url: URL, init: RequestInit) => Response | Promise<Response>;

function json(body: unknown, status = 200, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8", ...headers },
  });
}

function mock(handler: Handler) {
  const calls: { url: URL; init: RequestInit }[] = [];
  const fetch = vi.fn(async (input: string, init?: RequestInit) => {
    const url = new URL(input);
    calls.push({ url, init: init ?? {} });
    return handler(url, init ?? {});
  });
  return { fetch, calls };
}

const sword: Item = {
  id: "minecraft:diamond_sword",
  name: "Diamond Sword",
  kind: "flat",
  icon: "https://assets.lobbytick.com/games/minecraft/26.3/items/diamond_sword.png",
  sprite: [3, 2],
  max_stack: 1,
};

describe("requests", () => {
  it("lists games and sends the User-Agent", async () => {
    const { fetch, calls } = mock(() =>
      json({ games: [{ id: "minecraft", name: "Minecraft", versions: [{ version: "26.3", protocol: 777, items: 1658, languages: 145 }] }] }),
    );
    const c = new LobbyTick({ fetch });
    const games = await c.listGames();
    expect(games[0]!.id).toBe("minecraft");
    expect(calls[0]!.url.href).toBe("https://api.lobbytick.com/v1/games");
    const headers = calls[0]!.init.headers as Record<string, string>;
    expect(headers["User-Agent"]).toBe(`lobbytick-sdk-typescript/${VERSION}`);
    expect(headers.Accept).toBe("application/json");
  });

  it("builds every route, with latest as the default version", async () => {
    const { fetch, calls } = mock(() => json({}));
    const c = new LobbyTick({ fetch, baseUrl: "http://localhost:8080/" });
    await c.getGame("valheim");
    await c.getVersion("valheim");
    await c.listLanguages("valheim", "1.0.21");
    await c.listItems("minecraft", "latest", { lang: "pt_br", q: "espada diamante", kind: "flat", limit: 10, offset: 20 });
    await c.getItem("minecraft", "26.3", "minecraft:diamond_sword", { lang: "ja_jp" });
    expect(calls.map((x) => x.url.pathname + x.url.search)).toEqual([
      "/v1/games/valheim",
      "/v1/games/valheim/versions/latest",
      "/v1/games/valheim/versions/1.0.21/languages",
      "/v1/games/minecraft/versions/latest/items?lang=pt_br&q=espada+diamante&kind=flat&limit=10&offset=20",
      "/v1/games/minecraft/versions/26.3/items/minecraft:diamond_sword?lang=ja_jp",
    ]);
    expect(calls[0]!.url.origin).toBe("http://localhost:8080");
  });

  it("keeps ':' raw in item ids but escapes '/' and spaces", () => {
    expect(encodeSegment("minecraft:oak_log")).toBe("minecraft:oak_log");
    expect(encodeSegment("Base.3030Box")).toBe("Base.3030Box");
    expect(encodeSegment("a/b c")).toBe("a%2Fb%20c");
  });

  it("omits empty query parameters", async () => {
    const { fetch, calls } = mock(() => json({ items: [], total: 0 }));
    await new LobbyTick({ fetch }).listItems("zomboid", "latest", { q: "", lang: undefined });
    expect(calls[0]!.url.search).toBe("");
  });
});

describe("errors", () => {
  it("maps 404 and 400 to typed errors with the API message", async () => {
    const { fetch } = mock((url) =>
      url.pathname.endsWith("nope")
        ? json({ error: "unknown game" }, 404, { "Cache-Control": "no-store" })
        : json({ error: "unknown lang: see the version's /languages" }, 400),
    );
    const c = new LobbyTick({ fetch });
    const e1 = await c.getGame("nope").catch((e) => e);
    expect(e1).toBeInstanceOf(NotFoundError);
    expect(e1).toBeInstanceOf(APIError);
    expect(e1.status).toBe(404);
    expect(e1.message).toBe("unknown game");
    const e2 = await c.listItems("minecraft", "latest", { lang: "xx_xx" }).catch((e) => e);
    expect(e2).toBeInstanceOf(BadRequestError);
    expect(e2.message).toMatch(/unknown lang/);
  });

  it("keeps a non-JSON body as text", async () => {
    const { fetch } = mock(() => new Response("<html>bad gateway</html>", { status: 502 }));
    const e = await new LobbyTick({ fetch }).listGames().catch((x) => x);
    expect(e).toBeInstanceOf(APIError);
    expect(e.status).toBe(502);
    expect(e.message).toBe("HTTP 502");
    expect(e.body).toBe("<html>bad gateway</html>");
  });

  it("wraps network failures and timeouts", async () => {
    const down = new LobbyTick({ fetch: async () => { throw new TypeError("fetch failed"); } });
    await expect(down.listGames()).rejects.toBeInstanceOf(ConnectionError);

    const slow = new LobbyTick({
      timeout: 20,
      fetch: (_u, init) =>
        new Promise((_r, reject) => init!.signal!.addEventListener("abort", () => reject(new Error("aborted")))),
    });
    await expect(slow.listGames()).rejects.toBeInstanceOf(TimeoutError);
  });

  it("passes the caller's abort through untouched", async () => {
    const ac = new AbortController();
    const c = new LobbyTick({
      fetch: (_u, init) =>
        new Promise((_r, reject) => init!.signal!.addEventListener("abort", () => reject(init!.signal!.reason))),
    });
    const p = c.listGames({ signal: ac.signal });
    ac.abort(new Error("stop"));
    await expect(p).rejects.toThrow("stop");
  });
});

describe("rate limit", () => {
  const limited = () =>
    json({ error: "too many requests: wait and try again" }, 429, {
      "Retry-After": "7",
      "RateLimit-Limit": "120",
      "RateLimit-Remaining": "0",
      "RateLimit-Reset": "7",
    });

  it("throws RateLimitError with retryAfter by default (no retries)", async () => {
    const { fetch, calls } = mock(limited);
    const e = await new LobbyTick({ fetch }).listGames().catch((x) => x);
    expect(e).toBeInstanceOf(RateLimitError);
    expect(e.retryAfter).toBe(7);
    expect(e.rateLimit).toEqual({ limit: 120, remaining: 0, reset: 7 });
    expect(calls).toHaveLength(1);
  });

  it("retries after Retry-After when maxRetries is set", async () => {
    vi.useFakeTimers();
    try {
      let n = 0;
      const { fetch, calls } = mock(() => (n++ === 0 ? limited() : json({ games: [] })));
      const p = new LobbyTick({ fetch, maxRetries: 2 }).listGames();
      await vi.advanceTimersByTimeAsync(6_900);
      expect(calls).toHaveLength(1);
      await vi.advanceTimersByTimeAsync(200);
      await expect(p).resolves.toEqual([]);
      expect(calls).toHaveLength(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("gives up after maxRetries", async () => {
    const { fetch, calls } = mock(() => json({ error: "too many requests" }, 429, { "Retry-After": "0" }));
    await expect(new LobbyTick({ fetch, maxRetries: 2 }).listGames()).rejects.toBeInstanceOf(RateLimitError);
    expect(calls).toHaveLength(3);
  });

  it("does not sleep longer than maxRetryDelay", async () => {
    const { fetch, calls } = mock(() => json({ error: "too many requests" }, 429, { "Retry-After": "120" }));
    const e = await new LobbyTick({ fetch, maxRetries: 3, maxRetryDelay: 60 }).listGames().catch((x) => x);
    expect(e.retryAfter).toBe(120);
    expect(calls).toHaveLength(1);
  });

  it("parses Retry-After and documents the backoff", () => {
    expect(parseRetryAfter("5")).toBe(5);
    expect(parseRetryAfter(null)).toBeNull();
    expect(parseRetryAfter("soon")).toBeNull();
    const now = Date.parse("2026-11-21T12:00:00Z");
    expect(parseRetryAfter("Sat, 21 Nov 2026 12:00:10 GMT", now)).toBe(10);
    expect([0, 1, 2, 3, 4, 5, 6].map(backoffDelay)).toEqual([1, 2, 4, 8, 16, 30, 30]);
  });
});

describe("pagination", () => {
  it("walks every page and pins the version resolved by latest", async () => {
    const all = Array.from({ length: 5 }, (_, i) => ({ ...sword, id: `x:${i}`, name: `Item ${i}` }));
    const { fetch, calls } = mock((url) => {
      const limit = Number(url.searchParams.get("limit"));
      const offset = Number(url.searchParams.get("offset") ?? 0);
      return json({ version: "26.3", lang: "en_us", total: all.length, offset, limit, items: all.slice(offset, offset + limit) });
    });
    const got: string[] = [];
    for await (const it of new LobbyTick({ fetch }).iterItems("minecraft", "latest", { pageSize: 2, kind: "flat" })) got.push(it.id);
    expect(got).toEqual(["x:0", "x:1", "x:2", "x:3", "x:4"]);
    expect(calls).toHaveLength(3);
    expect(calls[0]!.url.pathname).toContain("/versions/latest/");
    expect(calls[1]!.url.pathname).toContain("/versions/26.3/");
    expect(calls[2]!.url.searchParams.get("offset")).toBe("4");
    expect(calls[2]!.url.searchParams.get("kind")).toBe("flat");
  });

  it("stops on an empty page and rejects bad page sizes", async () => {
    const { fetch, calls } = mock(() => json({ version: "1", lang: "en_us", total: 10, offset: 0, limit: 5, items: [] }));
    const c = new LobbyTick({ fetch });
    for await (const _ of c.iterItems("valheim")) throw new Error("no items expected");
    expect(calls).toHaveLength(1);
    await expect(c.iterItems("valheim", "latest", { pageSize: 5001 }).next()).rejects.toBeInstanceOf(RangeError);
  });
});

describe("icons", () => {
  const atlas = { url: "https://assets.lobbytick.com/games/minecraft/26.3/atlas.png", columns: 40, rows: 40, cell: 64 };

  it("computes the sprite rectangle", () => {
    expect(spriteRect(sword, atlas)).toEqual({ url: atlas.url, x: 192, y: 128, width: 64, height: 64 });
    expect(iconUrl(sword)).toBe(sword.icon);
  });

  it("returns null without sprite or atlas", () => {
    const chest: Item = { id: "minecraft:chest", name: "Chest", kind: "special", icon: null, sprite: null };
    expect(spriteRect(chest, atlas)).toBeNull();
    expect(spriteRect(sword, null)).toBeNull();
    expect(iconUrl(chest)).toBeNull();
  });
});
