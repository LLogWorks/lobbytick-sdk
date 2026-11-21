"""Errors raised by the client."""

from __future__ import annotations

import email.utils
import time
from dataclasses import dataclass
from typing import Any, Mapping, Optional

__all__ = [
    "APIError",
    "BadRequestError",
    "ConnectionError",
    "LobbyTickError",
    "NotFoundError",
    "RateLimitError",
    "RateLimitInfo",
    "TimeoutError",
]


class LobbyTickError(Exception):
    """Base class of every error this SDK raises."""


@dataclass(frozen=True)
class RateLimitInfo:
    """The ``RateLimit-*`` headers of a response; None when absent."""

    #: Requests allowed per window (120 per minute today).
    limit: Optional[int]
    #: Requests left in the current window.
    remaining: Optional[int]
    #: Seconds until the window resets.
    reset: Optional[int]

    @classmethod
    def from_headers(cls, headers: Mapping[str, str]) -> "RateLimitInfo":
        return cls(
            limit=_int(headers.get("ratelimit-limit")),
            remaining=_int(headers.get("ratelimit-remaining")),
            reset=_int(headers.get("ratelimit-reset")),
        )


class APIError(LobbyTickError):
    """The API answered with an HTTP error. ``message`` is the API's own ``error`` text."""

    def __init__(self, status: int, message: str, body: Any, headers: Mapping[str, str]) -> None:
        super().__init__(f"{status}: {message}")
        self.status = status
        self.message = message
        #: The parsed JSON body, or the raw text when it was not JSON.
        self.body = body
        #: Response headers, with lowercase names.
        self.headers = dict(headers)
        self.rate_limit = RateLimitInfo.from_headers(self.headers)


class BadRequestError(APIError):
    """400: a bad parameter (unknown ``lang``, ``limit`` out of 1..5000, ...)."""


class NotFoundError(APIError):
    """404: unknown game, no data for that version, or unknown item."""


class RateLimitError(APIError):
    """429: over the rate limit (120 requests per minute per visitor)."""

    def __init__(self, status: int, message: str, body: Any, headers: Mapping[str, str]) -> None:
        super().__init__(status, message, body, headers)
        retry = parse_retry_after(self.headers.get("retry-after"))
        #: Seconds to wait before trying again (``Retry-After``, else ``RateLimit-Reset``).
        self.retry_after: Optional[float] = retry if retry is not None else (
            float(self.rate_limit.reset) if self.rate_limit.reset is not None else None
        )


class ConnectionError(LobbyTickError):  # noqa: A001 - deliberately scoped to this package
    """The request never got an HTTP answer (DNS, refused connection, TLS...)."""


class TimeoutError(LobbyTickError):  # noqa: A001
    """The request took longer than the client's ``timeout``."""


def _int(raw: Optional[str]) -> Optional[int]:
    if raw is None:
        return None
    try:
        return int(raw.strip())
    except ValueError:
        return None


def parse_retry_after(raw: Optional[str], now: Optional[float] = None) -> Optional[float]:
    """Parse ``Retry-After`` as seconds or an HTTP date. None when absent or invalid."""
    if raw is None or not raw.strip():
        return None
    try:
        return max(0.0, float(raw))
    except ValueError:
        pass
    try:
        when = email.utils.parsedate_to_datetime(raw)
    except (TypeError, ValueError):
        return None
    if when is None:
        return None
    return max(0.0, float(int(when.timestamp() - (time.time() if now is None else now) + 0.999)))


def error_for(status: int, body: Any, headers: Mapping[str, str]) -> APIError:
    message = f"HTTP {status}"
    if isinstance(body, dict) and isinstance(body.get("error"), str):
        message = body["error"]
    cls = {400: BadRequestError, 404: NotFoundError, 429: RateLimitError}.get(status, APIError)
    return cls(status, message, body, headers)
