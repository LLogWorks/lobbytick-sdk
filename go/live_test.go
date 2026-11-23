package lobbytick

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// TestLive runs against the real API only with LOBBYTICK_LIVE=1 (a handful of requests).
func TestLive(t *testing.T) {
	if os.Getenv("LOBBYTICK_LIVE") != "1" {
		t.Skip("set LOBBYTICK_LIVE=1")
	}
	c, ctx := New(&Options{BaseURL: os.Getenv("LOBBYTICK_BASE_URL"), MaxRetries: 2}), context.Background()

	games, err := c.ListGames(ctx)
	if err != nil || len(games) == 0 {
		t.Fatalf("games: %v", err)
	}
	v, err := c.GetVersion(ctx, "minecraft", "latest")
	if err != nil || v.Atlas == nil || !strings.HasPrefix(v.Atlas.URL, "https://assets.lobbytick.com/") {
		t.Fatalf("version: %+v %v", v, err)
	}
	langs, err := c.ListLanguages(ctx, "valheim", "")
	if err != nil || langs.Default != "en_us" {
		t.Fatalf("languages: %v", err)
	}
	page, err := c.ListItems(ctx, "minecraft", v.Version, ListItemsParams{Lang: "pt_br", Q: "espada diamante", Limit: 5})
	if err != nil || len(page.Items) == 0 || page.Items[0].ID != "minecraft:diamond_sword" {
		t.Fatalf("search: %+v %v", page, err)
	}
	item, err := c.GetItem(ctx, "minecraft", v.Version, "minecraft:diamond_sword", "pt_br")
	if err != nil || item.Name != "Espada de Diamante" {
		t.Fatalf("item: %+v %v", item, err)
	}
	if r, ok := item.SpriteRect(v.Atlas); !ok || r.Width != v.Atlas.Cell {
		t.Fatalf("sprite: %+v", r)
	}
	n := 0
	for _, err := range c.IterItems(ctx, "valheim", "latest", IterItemsParams{PageSize: 5000}) {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n < 500 {
		t.Fatalf("valheim has only %d items", n)
	}
	if _, err := c.GetGame(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("nope: %v", err)
	}
}
