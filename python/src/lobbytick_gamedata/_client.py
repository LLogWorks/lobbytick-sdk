"""The LobbyTick game-data API client (stdlib only)."""

from __future__ import annotations

import json
import socket
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, Iterator, List, Mapping, Optional, Sequence

from ._errors import (
    APIError,
    ConnectionError,
    RateLimitError,
    RateLimitInfo,
    TimeoutError,
    error_for,
    parse_retry_after,
)
from ._models import Game, Item, ItemPage, LanguageList, Version
from ._version import __version__

__all__ = [
    "DEFAULT_BASE_URL",
    "MAX_PAGE_SIZE",
    "HTTPRequest",
    "HTTPResponse",
    "LobbyTick",
    "Transport",
    "UrllibTransport",
    "backoff_delay",
    "encode_segment",
]

DEFAULT_BASE_URL = "https://api.lobbytick.com"
#: The API's largest page.
MAX_PAGE_SIZE = 5000

_RETRY_STATUSES = frozenset({429, 502, 503, 504})


@dataclass(frozen=True)
class HTTPRequest:
    method: str
    url: str
    headers: Dict[str, str]
    #: Seconds; None for no timeout.
    timeout: Optional[float]


@dataclass(frozen=True)
class HTTPResponse:
    status: int
    #: Header names must be lowercase.
    headers: Dict[str, str]
    body: bytes = field(repr=False)


#: Anything that turns a request into a response: swap it for tests, proxies or
#: another HTTP library. It must raise ``TimeoutError``/``ConnectionError`` (from
#: this package) when there is no HTTP answer, and return HTTP errors as responses.
Transport = Callable[[HTTPRequest], HTTPResponse]


class UrllibTransport:
    """The default transport, on ``urllib.request``."""

    def __init__(self, opener: Optional[urllib.request.OpenerDirector] = None) -> None:
        self._opener = opener or urllib.request.build_opener()

    def __call__(self, request: HTTPRequest) -> HTTPResponse:
        req = urllib.request.Request(request.url, headers=request.headers, method=request.method)
        try:
            with self._opener.open(req, timeout=request.timeout) as res:
                return HTTPResponse(res.status, _lower(res.headers.items()), res.read())
        except urllib.error.HTTPError as e:
            with e:
                return HTTPResponse(e.code, _lower(e.headers.items() if e.headers else []), e.read())
        except socket.timeout as e:
            raise TimeoutError(f"request timed out after {request.timeout} s: {request.url}") from e
        except urllib.error.URLError as e:
            if isinstance(e.reason, socket.timeout):
                raise TimeoutError(f"request timed out after {request.timeout} s: {request.url}") from e
            raise ConnectionError(f"request failed: {request.url}: {e.reason}") from e
        except OSError as e:
            raise ConnectionError(f"request failed: {request.url}: {e}") from e


def _lower(items: Sequence[Any]) -> Dict[str, str]:
    return {k.lower(): v for k, v in items}


def backoff_delay(attempt: int) -> float:
    """Delay before retry number ``attempt`` (0-based) when the API gives no hint:
    1 s, 2 s, 4 s, 8 s... capped at 30 s."""
    return float(min(30, 2**attempt))


def encode_segment(s: str) -> str:
    """Encode one path segment, keeping ``:`` and ``@`` raw: the API matches item ids
    such as ``minecraft:diamond_sword`` on the raw path, so ``%3A`` would not be found."""
    return urllib.parse.quote(s, safe=":@!$&'()*+,;=")


class LobbyTick:
    """A client of the LobbyTick public game-data API. Read-only, no key needed.

    :param base_url: default ``https://api.lobbytick.com``.
    :param timeout: seconds per request (each retry gets its own); None disables.
    :param transport: a custom :data:`Transport` (default :class:`UrllibTransport`).
    :param max_retries: retries for a 429 (or 502/503/504). Default 0: the error is
        raised right away, carrying ``retry_after``.
    :param max_retry_delay: longest wait (seconds) accepted before a retry; if the
        API asks for more, the error is raised instead of sleeping. Default 60.
    :param user_agent: default ``lobbytick-sdk-python/<version>``.
    :param headers: extra headers on every request.
    :param sleep: how to wait between retries (default ``time.sleep``).
    """

    def __init__(
        self,
        base_url: str = DEFAULT_BASE_URL,
        *,
        timeout: Optional[float] = 30.0,
        transport: Optional[Transport] = None,
        max_retries: int = 0,
        max_retry_delay: float = 60.0,
        user_agent: Optional[str] = None,
        headers: Optional[Mapping[str, str]] = None,
        sleep: Callable[[float], None] = time.sleep,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.transport: Transport = transport or UrllibTransport()
        self.max_retries = max(0, max_retries)
        self.max_retry_delay = max_retry_delay
        self.user_agent = user_agent or f"lobbytick-sdk-python/{__version__}"
        self.headers = dict(headers or {})
        self._sleep = sleep
        #: Rate-limit headers of the last successful response.
        self.last_rate_limit: Optional[RateLimitInfo] = None

    # -- endpoints ---------------------------------------------------------

    def list_games(self) -> List[Game]:
        """Every game LobbyTick knows, with the versions it has data for."""
        return [Game.from_dict(g) for g in self._get(["v1", "games"])["games"]]

    def get_game(self, game: str) -> Game:
        """One game and its versions (newest first)."""
        return Game.from_dict(self._get(["v1", "games", game]))

    def get_version(self, game: str, version: str = "latest") -> Version:
        """One version's catalog summary and atlas. ``version`` may be ``"latest"``."""
        return Version.from_dict(self._get(["v1", "games", game, "versions", version]))

    def list_languages(self, game: str, version: str = "latest") -> LanguageList:
        """The languages item names exist in, for one version."""
        return LanguageList.from_dict(self._get(["v1", "games", game, "versions", version, "languages"]))

    def list_items(
        self,
        game: str,
        version: str = "latest",
        *,
        lang: Optional[str] = None,
        q: Optional[str] = None,
        kind: Optional[str] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
    ) -> ItemPage:
        """One page of items. ``limit`` is 1..5000 (server default 50). Every word of
        ``q`` must appear in the localized name, English name or id. ``empty`` items
        are never listed."""
        query = {"lang": lang, "q": q, "kind": kind, "limit": limit, "offset": offset}
        return ItemPage.from_dict(self._get(["v1", "games", game, "versions", version, "items"], query))

    def iter_items(
        self,
        game: str,
        version: str = "latest",
        *,
        lang: Optional[str] = None,
        q: Optional[str] = None,
        kind: Optional[str] = None,
        page_size: int = 1000,
    ) -> Iterator[Item]:
        """Every matching item, fetching pages as needed. With ``"latest"``, the version
        of the first page is used for the rest, so a deploy mid-way cannot mix catalogs."""
        if not 1 <= page_size <= MAX_PAGE_SIZE:
            raise ValueError(f"page_size must be between 1 and {MAX_PAGE_SIZE}")
        offset = 0
        while True:
            page = self.list_items(game, version, lang=lang, q=q, kind=kind, limit=page_size, offset=offset)
            version = page.version
            yield from page.items
            offset += len(page.items)
            if not page.items or offset >= page.total:
                return

    def get_item(self, game: str, version: str, id: str, *, lang: Optional[str] = None) -> Item:  # noqa: A002
        """One item by its exact id, named in ``lang``."""
        return Item.from_dict(self._get(["v1", "games", game, "versions", version, "items", id], {"lang": lang}))

    # -- plumbing ----------------------------------------------------------

    def _url(self, path: Sequence[str], query: Optional[Mapping[str, Any]]) -> str:
        url = self.base_url + "/" + "/".join(encode_segment(p) for p in path)
        params = [(k, str(v)) for k, v in (query or {}).items() if v is not None and v != ""]
        if params:
            url += "?" + urllib.parse.urlencode(params)
        return url

    def _get(self, path: Sequence[str], query: Optional[Mapping[str, Any]] = None) -> Any:
        url = self._url(path, query)
        headers = {"Accept": "application/json", "User-Agent": self.user_agent, **self.headers}
        attempt = 0
        while True:
            res = self.transport(HTTPRequest("GET", url, headers, self.timeout))
            if 200 <= res.status < 300:
                self.last_rate_limit = RateLimitInfo.from_headers(res.headers)
                return json.loads(res.body)
            text = res.body.decode("utf-8", "replace")
            try:
                body: Any = json.loads(text)
            except ValueError:
                body = text
            err: APIError = error_for(res.status, body, res.headers)
            if attempt >= self.max_retries or res.status not in _RETRY_STATUSES:
                raise err
            if isinstance(err, RateLimitError):
                hinted = err.retry_after
            else:
                hinted = parse_retry_after(res.headers.get("retry-after"))
            delay = hinted if hinted is not None else backoff_delay(attempt)
            if delay > self.max_retry_delay:
                raise err
            self._sleep(delay)
            attempt += 1

    def __repr__(self) -> str:
        return f"LobbyTick(base_url={self.base_url!r})"
