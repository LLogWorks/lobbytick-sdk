/** Base class of every error this SDK throws. */
export class LobbyTickError extends Error {
  constructor(message: string, options?: { cause?: unknown }) {
    super(message, options);
    this.name = new.target.name;
  }
}

/** Rate-limit headers of a response (`RateLimit-*`). Null when absent. */
export interface RateLimitInfo {
  /** Requests allowed per window (120 per minute today). */
  limit: number | null;
  /** Requests left in the current window. */
  remaining: number | null;
  /** Seconds until the window resets. */
  reset: number | null;
}

/** The API answered with an HTTP error. `message` is the API's own `error` text. */
export class APIError extends LobbyTickError {
  readonly status: number;
  /** The parsed JSON body, or the raw text when it was not JSON. */
  readonly body: unknown;
  readonly headers: Headers;
  readonly rateLimit: RateLimitInfo;

  constructor(status: number, message: string, body: unknown, headers: Headers) {
    super(message);
    this.status = status;
    this.body = body;
    this.headers = headers;
    this.rateLimit = rateLimitInfo(headers);
  }
}

/** 400: a bad parameter (unknown `lang`, `limit` out of 1..5000, ...). */
export class BadRequestError extends APIError {}

/** 404: unknown game, no data for that version, or unknown item. */
export class NotFoundError extends APIError {}

/** 429: over the rate limit (120 requests per minute per visitor). */
export class RateLimitError extends APIError {
  /** Seconds to wait before trying again, from `Retry-After` (or `RateLimit-Reset`). */
  readonly retryAfter: number | null;

  constructor(status: number, message: string, body: unknown, headers: Headers) {
    super(status, message, body, headers);
    this.retryAfter = parseRetryAfter(headers.get("retry-after")) ?? this.rateLimit.reset;
  }
}

/** The request never got an HTTP answer (DNS, connection refused, CORS...). */
export class ConnectionError extends LobbyTickError {}

/** The request took longer than the client's `timeout`. */
export class TimeoutError extends LobbyTickError {}

function intHeader(headers: Headers, name: string): number | null {
  const raw = headers.get(name);
  if (raw === null || raw.trim() === "") return null;
  const n = Number(raw);
  return Number.isFinite(n) ? n : null;
}

export function rateLimitInfo(headers: Headers): RateLimitInfo {
  return {
    limit: intHeader(headers, "ratelimit-limit"),
    remaining: intHeader(headers, "ratelimit-remaining"),
    reset: intHeader(headers, "ratelimit-reset"),
  };
}

/** Parses `Retry-After` as seconds or an HTTP date. Null when absent or invalid. */
export function parseRetryAfter(raw: string | null, now: number = Date.now()): number | null {
  if (raw === null || raw.trim() === "") return null;
  const n = Number(raw);
  if (Number.isFinite(n)) return Math.max(0, n);
  const date = Date.parse(raw);
  if (Number.isNaN(date)) return null;
  return Math.max(0, Math.ceil((date - now) / 1000));
}

export function errorFor(status: number, body: unknown, headers: Headers): APIError {
  let message = `HTTP ${status}`;
  if (body && typeof body === "object" && typeof (body as { error?: unknown }).error === "string") {
    message = (body as { error: string }).error;
  }
  switch (status) {
    case 400:
      return new BadRequestError(status, message, body, headers);
    case 404:
      return new NotFoundError(status, message, body, headers);
    case 429:
      return new RateLimitError(status, message, body, headers);
    default:
      return new APIError(status, message, body, headers);
  }
}
