package lobbytick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu    sync.Mutex
	paths []string
	reqs  []*http.Request
}

func server(t *testing.T, h func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *recorder) {
	t.Helper()
	rec := &recorder{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		p := r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			p += "?" + r.URL.RawQuery
		}
		rec.paths = append(rec.paths, p)
		rec.reqs = append(rec.reqs, r)
		rec.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(s.Close)
	return s, rec
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func noSleep(slept *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, d time.Duration) error { *slept = append(*slept, d); return nil }
}

func TestListGamesAndUserAgent(t *testing.T) {
	s, rec := server(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"games": []any{map[string]any{"id": "minecraft", "name": "Minecraft",
			"versions": []any{map[string]any{"version": "26.3", "protocol": 777, "items": 1658, "languages": 145}}}}})
	})
	games, err := New(&Options{BaseURL: s.URL + "/"}).ListGames(context.Background())
	if err != nil || len(games) != 1 || games[0].Versions[0].Items != 1658 {
		t.Fatalf("games = %+v, %v", games, err)
	}
	if ua := rec.reqs[0].Header.Get("User-Agent"); ua != "lobbytick-sdk-go/"+SDKVersion {
		t.Fatalf("User-Agent = %q", ua)
	}
	if rec.paths[0] != "/v1/games" {
		t.Fatalf("path = %s", rec.paths[0])
	}
}

func TestRoutes(t *testing.T) {
	s, rec := server(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{}) })
	c, ctx := New(&Options{BaseURL: s.URL}), context.Background()
	must := func(_ any, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(c.GetGame(ctx, "valheim"))
	must(c.GetVersion(ctx, "valheim", ""))
	must(c.ListLanguages(ctx, "valheim", "1.0.21"))
	must(c.ListItems(ctx, "minecraft", "latest", ListItemsParams{Lang: "pt_br", Q: "espada diamante", Kind: KindFlat, Limit: 10, Offset: 20}))
	must(c.GetItem(ctx, "minecraft", "26.3", "minecraft:diamond_sword", "ja_jp"))
	want := []string{
		"/v1/games/valheim",
		"/v1/games/valheim/versions/latest",
		"/v1/games/valheim/versions/1.0.21/languages",
		"/v1/games/minecraft/versions/latest/items?kind=flat&lang=pt_br&limit=10&offset=20&q=espada+diamante",
		"/v1/games/minecraft/versions/26.3/items/minecraft:diamond_sword?lang=ja_jp",
	}
	if fmt.Sprint(rec.paths) != fmt.Sprint(want) {
		t.Fatalf("paths:\n%v\nwant:\n%v", rec.paths, want)
	}
	if EscapeSegment("a/b c") != "a%2Fb%20c" || EscapeSegment("Base.3030Box") != "Base.3030Box" {
		t.Fatal("EscapeSegment")
	}
}

func TestTypedErrors(t *testing.T) {
	s, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/games/nope":
			writeJSON(w, 404, map[string]string{"error": "unknown game"})
		case "/v1/games/bad":
			w.WriteHeader(502)
			_, _ = w.Write([]byte("<html>bad gateway</html>"))
		default:
			writeJSON(w, 400, map[string]string{"error": "unknown lang: see the version's /languages"})
		}
	})
	c, ctx := New(&Options{BaseURL: s.URL}), context.Background()

	_, err := c.GetGame(ctx, "nope")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 || apiErr.Message != "unknown game" || !errors.Is(err, ErrNotFound) {
		t.Fatalf("404 = %v", err)
	}
	_, err = c.ListItems(ctx, "minecraft", "", ListItemsParams{Lang: "xx_xx"})
	if !errors.Is(err, ErrBadRequest) || errors.Is(err, ErrNotFound) {
		t.Fatalf("400 = %v", err)
	}
	_, err = c.GetGame(ctx, "bad")
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 502 || apiErr.Message != "HTTP 502" || string(apiErr.Body) != "<html>bad gateway</html>" {
		t.Fatalf("502 = %v", err)
	}
}

func TestContextAndTimeout(t *testing.T) {
	s, _ := server(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := New(&Options{BaseURL: s.URL}).ListGames(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ctx = %v", err)
	}
	if _, err := New(&Options{BaseURL: s.URL, Timeout: 20 * time.Millisecond}).ListGames(context.Background()); err == nil {
		t.Fatal("timeout expected")
	}
}

func limited(w http.ResponseWriter, retryAfter string) {
	w.Header().Set("RateLimit-Limit", "120")
	w.Header().Set("RateLimit-Remaining", "0")
	w.Header().Set("RateLimit-Reset", retryAfter)
	w.Header().Set("Retry-After", retryAfter)
	writeJSON(w, 429, map[string]string{"error": "too many requests: wait and try again"})
}

func TestRateLimitDefaultReturnsError(t *testing.T) {
	s, rec := server(t, func(w http.ResponseWriter, r *http.Request) { limited(w, "7") })
	_, err := New(&Options{BaseURL: s.URL}).ListGames(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !errors.Is(err, ErrRateLimited) || apiErr.RetryAfter != 7*time.Second {
		t.Fatalf("429 = %v", err)
	}
	if apiErr.RateLimit.Limit != 120 || apiErr.RateLimit.Remaining != 0 || apiErr.RateLimit.Reset != 7*time.Second {
		t.Fatalf("rate limit = %+v", apiErr.RateLimit)
	}
	if len(rec.paths) != 1 {
		t.Fatalf("%d requests", len(rec.paths))
	}
}

func TestRateLimitRetries(t *testing.T) {
	n := 0
	s, rec := server(t, func(w http.ResponseWriter, r *http.Request) {
		if n++; n == 1 {
			limited(w, "7")
			return
		}
		writeJSON(w, 200, map[string]any{"games": []any{}})
	})
	var slept []time.Duration
	c := New(&Options{BaseURL: s.URL, MaxRetries: 2})
	c.sleep = noSleep(&slept)
	if _, err := c.ListGames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rec.paths) != 2 || fmt.Sprint(slept) != "[7s]" {
		t.Fatalf("requests %d, slept %v", len(rec.paths), slept)
	}
}

func TestBackoffAndCap(t *testing.T) {
	s, rec := server(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 503, map[string]string{"error": "busy"}) })
	var slept []time.Duration
	c := New(&Options{BaseURL: s.URL, MaxRetries: 3})
	c.sleep = noSleep(&slept)
	if _, err := c.ListGames(context.Background()); err == nil {
		t.Fatal("error expected")
	}
	if len(rec.paths) != 4 || fmt.Sprint(slept) != "[1s 2s 4s]" {
		t.Fatalf("requests %d, slept %v", len(rec.paths), slept)
	}

	s2, rec2 := server(t, func(w http.ResponseWriter, r *http.Request) { limited(w, "120") })
	c2 := New(&Options{BaseURL: s2.URL, MaxRetries: 3})
	c2.sleep = noSleep(&slept)
	if _, err := c2.ListGames(context.Background()); !errors.Is(err, ErrRateLimited) || len(rec2.paths) != 1 {
		t.Fatalf("over the cap: %v after %d requests", err, len(rec2.paths))
	}

	var got []time.Duration
	for i := range 7 {
		got = append(got, BackoffDelay(i))
	}
	if fmt.Sprint(got) != "[1s 2s 4s 8s 16s 30s 30s]" {
		t.Fatalf("backoff = %v", got)
	}
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	if d, ok := ParseRetryAfter("Sat, 21 Nov 2026 12:00:10 GMT", now); !ok || d != 10*time.Second {
		t.Fatalf("http date = %v %v", d, ok)
	}
	if _, ok := ParseRetryAfter("soon", now); ok {
		t.Fatal("invalid accepted")
	}
}

func TestIterItems(t *testing.T) {
	s, rec := server(t, func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		var items []Item
		for i := offset; i < 5 && len(items) < limit; i++ {
			items = append(items, Item{ID: fmt.Sprintf("x:%d", i), Kind: KindFlat})
		}
		writeJSON(w, 200, ItemPage{Version: "26.3", Lang: "en_us", Total: 5, Offset: offset, Limit: limit, Items: items})
	})
	c := New(&Options{BaseURL: s.URL})
	var ids []string
	for it, err := range c.IterItems(context.Background(), "minecraft", "latest", IterItemsParams{PageSize: 2, Kind: KindFlat}) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, it.ID)
	}
	if fmt.Sprint(ids) != "[x:0 x:1 x:2 x:3 x:4]" || len(rec.paths) != 3 {
		t.Fatalf("ids %v after %d requests", ids, len(rec.paths))
	}
	if rec.paths[1] != "/v1/games/minecraft/versions/26.3/items?kind=flat&limit=2&offset=2" {
		t.Fatalf("second page = %s", rec.paths[1])
	}
	// Stopping early makes no more requests.
	for range c.IterItems(context.Background(), "minecraft", "", IterItemsParams{PageSize: 2}) {
		break
	}
	if len(rec.paths) != 4 {
		t.Fatalf("break: %d requests", len(rec.paths))
	}
	for _, err := range c.IterItems(context.Background(), "minecraft", "", IterItemsParams{PageSize: 5001}) {
		if err == nil {
			t.Fatal("bad page size accepted")
		}
	}
}

func TestSpriteHelpers(t *testing.T) {
	icon := "https://a/diamond_sword.png"
	sword := Item{ID: "minecraft:diamond_sword", Icon: &icon, Sprite: &[2]int{3, 2}}
	atlas := &Atlas{URL: "https://a/atlas.png", Columns: 40, Rows: 40, Cell: 64}
	r, ok := sword.SpriteRect(atlas)
	if !ok || r != (SpriteRect{URL: atlas.URL, X: 192, Y: 128, Width: 64, Height: 64}) || sword.IconURL() != icon {
		t.Fatalf("rect = %+v", r)
	}
	chest := Item{ID: "minecraft:chest", Kind: KindSpecial}
	if _, ok := chest.SpriteRect(atlas); ok || chest.IconURL() != "" {
		t.Fatal("chest has no icon")
	}
	if _, ok := sword.SpriteRect(nil); ok {
		t.Fatal("no atlas")
	}
}
