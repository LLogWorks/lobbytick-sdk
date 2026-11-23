import {
  ConnectionError,
  LobbyTickError,
  RateLimitError,
  TimeoutError,
  errorFor,
  parseRetryAfter,
  type APIError,
} from "./errors.js";
import type { Game, GameList, Item, ItemKind, ItemPage, LanguageList, Version } from "./types.js";
import { VERSION } from "./version.js";

export const DEFAULT_BASE_URL = "https://api.lobbytick.com";
/** The API's largest page. */
export const MAX_PAGE_SIZE = 5000;

type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

export interface ClientOptions {
  /** Default `https://api.lobbytick.com`. */
  baseUrl?: string;
  /** Per-request timeout in milliseconds (each retry gets its own). Default 30000; 0 disables. */
  timeout?: number;
  /** A custom `fetch` (default: the global one). */
  fetch?: FetchLike;
  /**
   * How many times to retry a request answered with 429 (or 502/503/504).
   * Default 0: the error is thrown right away, carrying `retryAfter`.
   */
  maxRetries?: number;
  /**
   * Longest wait (in seconds) the client accepts before a retry. If the API asks
   * for more, the error is thrown instead of sleeping. Default 60.
   */
  maxRetryDelay?: number;
  /**
   * Sent as `User-Agent` outside browsers. Default `lobbytick-sdk-typescript/<version>`.
   * Browsers never get it: a custom User-Agent would force a CORS preflight.
   */
  userAgent?: string;
  /** Extra headers on every request. */
  headers?: Record<string, string>;
}

export interface RequestOptions {
  /** Cancels the request (and any pending retry). */
  signal?: AbortSignal;
}

export interface LangOptions extends RequestOptions {
  /** Language code from `listLanguages`, e.g. `pt_br`. Default `en_us`. */
  lang?: string;
}

export interface ListItemsOptions extends LangOptions {
  /** Every word must appear in the localized name, English name or id (case-insensitive). Max 100 chars. */
  q?: string;
  kind?: ItemKind;
  /** 1..5000, default 50 (server side). */
  limit?: number;
  offset?: number;
}

export interface IterItemsOptions extends Omit<ListItemsOptions, "limit" | "offset"> {
  /** Items per request, 1..5000. Default 1000. */
  pageSize?: number;
}

const RETRY_STATUSES = new Set([429, 502, 503, 504]);

/**
 * The delay before retry number `attempt` (0-based) when the API gives no hint:
 * 1 s, 2 s, 4 s, 8 s... capped at 30 s.
 */
export function backoffDelay(attempt: number): number {
  return Math.min(30, 2 ** attempt);
}

function isBrowser(): boolean {
  const g = globalThis as { window?: unknown; document?: unknown };
  return typeof g.window !== "undefined" && typeof g.document !== "undefined";
}

/**
 * Encodes a path segment. `:` and `@` stay as they are: the API matches item
 * ids such as `minecraft:diamond_sword` on the raw path, so `%3A` would not be found.
 */
export function encodeSegment(s: string): string {
  return encodeURIComponent(s).replace(/%3A/gi, ":").replace(/%40/gi, "@");
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(signal.reason);
    const t = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    const onAbort = () => {
      clearTimeout(t);
      reject(signal!.reason);
    };
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

/** A client of the LobbyTick public game-data API. Read-only, no key needed. */
export class LobbyTick {
  readonly baseUrl: string;
  readonly timeout: number;
  readonly maxRetries: number;
  readonly maxRetryDelay: number;
  readonly userAgent: string;
  private readonly fetchImpl: FetchLike;
  private readonly extraHeaders: Record<string, string>;

  constructor(options: ClientOptions = {}) {
    this.baseUrl = (options.baseUrl ?? DEFAULT_BASE_URL).replace(/\/+$/, "");
    this.timeout = options.timeout ?? 30_000;
    this.maxRetries = Math.max(0, options.maxRetries ?? 0);
    this.maxRetryDelay = options.maxRetryDelay ?? 60;
    this.userAgent = options.userAgent ?? `lobbytick-sdk-typescript/${VERSION}`;
    this.extraHeaders = options.headers ?? {};
    const f = options.fetch ?? (globalThis.fetch as FetchLike | undefined);
    if (!f) throw new LobbyTickError("no fetch available: pass options.fetch");
    // Call the global fetch unbound-safe (some runtimes require `this === globalThis`).
    this.fetchImpl = options.fetch ? f : (input, init) => globalThis.fetch(input, init);
  }

  /** Every game LobbyTick knows, with the versions it has data for. */
  async listGames(options: RequestOptions = {}): Promise<Game[]> {
    const out = await this.request<GameList>(["v1", "games"], {}, options);
    return out.games;
  }

  /** One game and its versions (newest first). */
  getGame(game: string, options: RequestOptions = {}): Promise<Game> {
    return this.request<Game>(["v1", "games", game], {}, options);
  }

  /** One version's catalog summary and atlas. `version` may be `"latest"`. */
  getVersion(game: string, version = "latest", options: RequestOptions = {}): Promise<Version> {
    return this.request<Version>(["v1", "games", game, "versions", version], {}, options);
  }

  /** The languages item names exist in, for one version. */
  listLanguages(game: string, version = "latest", options: RequestOptions = {}): Promise<LanguageList> {
    return this.request<LanguageList>(["v1", "games", game, "versions", version, "languages"], {}, options);
  }

  /** One page of items. `empty` items (air) are never listed. */
  listItems(game: string, version = "latest", options: ListItemsOptions = {}): Promise<ItemPage> {
    const { lang, q, kind, limit, offset, signal } = options;
    return this.request<ItemPage>(
      ["v1", "games", game, "versions", version, "items"],
      { lang, q, kind, limit, offset },
      { signal },
    );
  }

  /**
   * Every matching item, fetching pages as needed. When `version` is `"latest"`,
   * the version of the first page is used for the rest, so a deploy mid-way
   * cannot mix two catalogs.
   */
  async *iterItems(game: string, version = "latest", options: IterItemsOptions = {}): AsyncGenerator<Item, void, undefined> {
    const { pageSize = 1000, ...rest } = options;
    if (!Number.isInteger(pageSize) || pageSize < 1 || pageSize > MAX_PAGE_SIZE) {
      throw new RangeError(`pageSize must be between 1 and ${MAX_PAGE_SIZE}`);
    }
    let offset = 0;
    let v = version;
    for (;;) {
      const page = await this.listItems(game, v, { ...rest, limit: pageSize, offset });
      v = page.version;
      yield* page.items;
      offset += page.items.length;
      if (page.items.length === 0 || offset >= page.total) return;
    }
  }

  /** One item by its exact id, named in `lang`. */
  getItem(game: string, version: string, id: string, options: LangOptions = {}): Promise<Item> {
    const { lang, signal } = options;
    return this.request<Item>(["v1", "games", game, "versions", version, "items", id], { lang }, { signal });
  }

  private url(path: string[], query: Record<string, string | number | undefined>): string {
    let url = this.baseUrl + "/" + path.map(encodeSegment).join("/");
    const params = new URLSearchParams();
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== null && v !== "") params.set(k, String(v));
    }
    const qs = params.toString();
    if (qs) url += "?" + qs;
    return url;
  }

  private async request<T>(
    path: string[],
    query: Record<string, string | number | undefined>,
    options: RequestOptions,
  ): Promise<T> {
    const url = this.url(path, query);
    const headers: Record<string, string> = { Accept: "application/json", ...this.extraHeaders };
    if (!isBrowser()) headers["User-Agent"] = this.userAgent;

    for (let attempt = 0; ; attempt++) {
      const res = await this.send(url, headers, options.signal);
      if (res.ok) return (await res.json()) as T;

      const text = await res.text();
      let body: unknown = text;
      try {
        body = JSON.parse(text);
      } catch {
        // not JSON (e.g. a proxy's HTML page): keep the text
      }
      const err: APIError = errorFor(res.status, body, res.headers);
      if (attempt >= this.maxRetries || !RETRY_STATUSES.has(res.status)) throw err;
      const hinted =
        err instanceof RateLimitError
          ? err.retryAfter
          : parseRetryAfter(res.headers.get("retry-after"));
      const delay = hinted ?? backoffDelay(attempt);
      if (delay > this.maxRetryDelay) throw err;
      await sleep(delay * 1000, options.signal);
    }
  }

  private async send(url: string, headers: Record<string, string>, signal?: AbortSignal): Promise<Response> {
    const controller = new AbortController();
    let timedOut = false;
    const timer =
      this.timeout > 0
        ? setTimeout(() => {
            timedOut = true;
            controller.abort();
          }, this.timeout)
        : undefined;
    const onAbort = () => controller.abort(signal!.reason);
    if (signal?.aborted) controller.abort(signal.reason);
    signal?.addEventListener("abort", onAbort, { once: true });
    try {
      return await this.fetchImpl(url, { method: "GET", headers, signal: controller.signal });
    } catch (e) {
      if (timedOut) throw new TimeoutError(`request timed out after ${this.timeout} ms: ${url}`, { cause: e });
      if (signal?.aborted) throw e;
      throw new ConnectionError(`request failed: ${url}: ${(e as Error)?.message ?? e}`, { cause: e });
    } finally {
      if (timer !== undefined) clearTimeout(timer);
      signal?.removeEventListener("abort", onAbort);
    }
  }
}
