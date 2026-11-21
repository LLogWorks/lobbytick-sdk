// Package lobbytick is a client of the LobbyTick public game-data API
// (https://api.lobbytick.com): the items of Minecraft, Valheim and Project
// Zomboid per game version, their names in every language and their icons.
//
// The API is read-only and needs no key. It allows 120 requests per minute per
// visitor; see [Options.MaxRetries] for what the client does when that is exceeded.
package lobbytick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SDKVersion is this SDK's version, sent in the User-Agent.
const SDKVersion = "0.1.0"

// DefaultBaseURL is the public API.
const DefaultBaseURL = "https://api.lobbytick.com"

// MaxPageSize is the API's largest page.
const MaxPageSize = 5000

// Options configure a [Client]. The zero value is ready to use.
type Options struct {
	// BaseURL defaults to DefaultBaseURL.
	BaseURL string
	// HTTPClient defaults to a client with Timeout.
	HTTPClient *http.Client
	// Timeout per request when HTTPClient is nil. Default 30 s; negative disables.
	Timeout time.Duration
	// MaxRetries is how many times a request answered with 429 (or 502/503/504)
	// is retried. Default 0: the *APIError is returned right away, carrying RetryAfter.
	MaxRetries int
	// MaxRetryDelay is the longest wait accepted before a retry; if the API asks
	// for more, the error is returned instead. Default 60 s.
	MaxRetryDelay time.Duration
	// UserAgent defaults to "lobbytick-sdk-go/<SDKVersion>".
	UserAgent string
}

// Client talks to the API. It is safe for concurrent use.
type Client struct {
	baseURL       string
	http          *http.Client
	maxRetries    int
	maxRetryDelay time.Duration
	userAgent     string
	sleep         func(context.Context, time.Duration) error
}

// New builds a client; opts may be nil.
func New(opts *Options) *Client {
	var o Options
	if opts != nil {
		o = *opts
	}
	c := &Client{
		baseURL:       strings.TrimRight(o.BaseURL, "/"),
		http:          o.HTTPClient,
		maxRetries:    max(0, o.MaxRetries),
		maxRetryDelay: o.MaxRetryDelay,
		userAgent:     o.UserAgent,
		sleep:         sleepCtx,
	}
	if c.baseURL == "" {
		c.baseURL = DefaultBaseURL
	}
	if c.http == nil {
		timeout := o.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		} else if timeout < 0 {
			timeout = 0
		}
		c.http = &http.Client{Timeout: timeout}
	}
	if c.maxRetryDelay == 0 {
		c.maxRetryDelay = 60 * time.Second
	}
	if c.userAgent == "" {
		c.userAgent = "lobbytick-sdk-go/" + SDKVersion
	}
	return c
}

// ListGames returns every game LobbyTick knows, with the versions it has data for.
func (c *Client) ListGames(ctx context.Context) ([]Game, error) {
	var out struct {
		Games []Game `json:"games"`
	}
	if err := c.get(ctx, []string{"v1", "games"}, nil, &out); err != nil {
		return nil, err
	}
	return out.Games, nil
}

// GetGame returns one game and its versions (newest first).
func (c *Client) GetGame(ctx context.Context, game string) (*Game, error) {
	var out Game
	if err := c.get(ctx, []string{"v1", "games", game}, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVersion returns one version's summary and atlas; version may be "latest"
// (or "", which means the same).
func (c *Client) GetVersion(ctx context.Context, game, version string) (*Version, error) {
	var out Version
	if err := c.get(ctx, []string{"v1", "games", game, "versions", orLatest(version)}, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListLanguages returns the languages item names exist in, for one version.
func (c *Client) ListLanguages(ctx context.Context, game, version string) (*LanguageList, error) {
	var out LanguageList
	if err := c.get(ctx, []string{"v1", "games", game, "versions", orLatest(version), "languages"}, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListItemsParams filter ListItems. Zero values are not sent.
type ListItemsParams struct {
	// Lang is a code from ListLanguages, e.g. "pt_br". Default "en_us".
	Lang string
	// Q: every word must appear in the localized name, English name or id
	// (case-insensitive). At most 100 bytes.
	Q string
	// Kind is one of the Kind* constants.
	Kind string
	// Limit is 1..5000; 0 means the server default (50).
	Limit  int
	Offset int
}

func (p ListItemsParams) query() url.Values {
	q := url.Values{}
	if p.Lang != "" {
		q.Set("lang", p.Lang)
	}
	if p.Q != "" {
		q.Set("q", p.Q)
	}
	if p.Kind != "" {
		q.Set("kind", p.Kind)
	}
	if p.Limit != 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset != 0 {
		q.Set("offset", strconv.Itoa(p.Offset))
	}
	return q
}

// ListItems returns one page of items. Empty items (air) are never listed.
func (c *Client) ListItems(ctx context.Context, game, version string, p ListItemsParams) (*ItemPage, error) {
	var out ItemPage
	if err := c.get(ctx, []string{"v1", "games", game, "versions", orLatest(version), "items"}, p.query(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetItem returns one item by its exact id, named in lang ("" for English).
func (c *Client) GetItem(ctx context.Context, game, version, id, lang string) (*Item, error) {
	q := url.Values{}
	if lang != "" {
		q.Set("lang", lang)
	}
	var out Item
	if err := c.get(ctx, []string{"v1", "games", game, "versions", orLatest(version), "items", id}, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func orLatest(v string) string {
	if v == "" {
		return "latest"
	}
	return v
}

// BackoffDelay is the wait before retry number attempt (0-based) when the API
// gives no hint: 1 s, 2 s, 4 s, 8 s... capped at 30 s.
func BackoffDelay(attempt int) time.Duration {
	if attempt >= 5 {
		return 30 * time.Second
	}
	return min(30*time.Second, time.Duration(1<<attempt)*time.Second)
}

// EscapeSegment escapes one path segment, keeping ":" and "@" raw: the API
// matches item ids like "minecraft:diamond_sword" on the raw path, so "%3A"
// would not be found.
func EscapeSegment(s string) string {
	return url.PathEscape(s)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Client) get(ctx context.Context, path []string, query url.Values, into any) error {
	segs := make([]string, len(path))
	for i, p := range path {
		segs[i] = EscapeSegment(p)
	}
	u := c.baseURL + "/" + strings.Join(segs, "/")
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	for attempt := 0; ; attempt++ {
		err := c.do(ctx, u, into)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || attempt >= c.maxRetries {
			return err
		}
		var delay time.Duration
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests:
			delay = apiErr.RetryAfter
			if !apiErr.hinted {
				delay = BackoffDelay(attempt)
			}
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			if d, ok := ParseRetryAfter(apiErr.Header.Get("Retry-After"), time.Now()); ok {
				delay = d
			} else {
				delay = BackoffDelay(attempt)
			}
		default:
			return err
		}
		if delay > c.maxRetryDelay {
			return err
		}
		if serr := c.sleep(ctx, delay); serr != nil {
			return serr
		}
	}
}

func (c *Client) do(ctx context.Context, u string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("lobbytick: GET %s: %w", u, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("lobbytick: GET %s: reading body: %w", u, err)
	}
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		if err := json.Unmarshal(body, into); err != nil {
			return fmt.Errorf("lobbytick: GET %s: decoding: %w", u, err)
		}
		return nil
	}
	e := &APIError{StatusCode: res.StatusCode, Message: "HTTP " + strconv.Itoa(res.StatusCode), Body: body, Header: res.Header, RateLimit: rateLimitOf(res.Header)}
	var msg struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &msg) == nil && msg.Error != "" {
		e.Message = msg.Error
	}
	if res.StatusCode == http.StatusTooManyRequests {
		e.RetryAfter, e.hinted = retryHint(res.Header)
	}
	return e
}
