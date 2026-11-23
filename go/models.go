package lobbytick

// Item kinds: how an item's icon was made, which is also roughly what the item is.
const (
	// KindFlat has a 2D icon: tools, food, ingots.
	KindFlat = "flat"
	// KindBlock has its block drawn in 3D.
	KindBlock = "block"
	// KindSpecial is drawn by the game's own code: it has no icon yet.
	KindSpecial = "special"
	// KindEmpty is not something a player can hold (air); never listed.
	KindEmpty = "empty"
)

// VersionSummary is one version of a game, as listed in a [Game].
type VersionSummary struct {
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
	// Items is how many items the catalog has (including empty ones).
	Items     int `json:"items"`
	Languages int `json:"languages"`
}

// Game is one game and the versions LobbyTick has data for, newest first.
type Game struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Versions []VersionSummary `json:"versions"`
}

// Atlas is every icon of a version in one image: a grid of square cells.
type Atlas struct {
	URL     string `json:"url"`
	Columns int    `json:"columns"`
	Rows    int    `json:"rows"`
	// Cell is the side of each cell, in pixels.
	Cell int `json:"cell"`
}

// Version is one version's catalog summary.
type Version struct {
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
	// WorldVersion is 0 when the game has no such thing.
	WorldVersion int `json:"world_version"`
	Items        int `json:"items"`
	Languages    int `json:"languages"`
	// IconSize is the side of each icon, in pixels.
	IconSize int    `json:"icon_size"`
	Atlas    *Atlas `json:"atlas"`
}

// Language is one language item names exist in.
type Language struct {
	// Code is the game's own, e.g. "pt_br".
	Code   string `json:"code"`
	Name   string `json:"name"`
	Region string `json:"region"`
	// Items is how many items it names; the rest fall back to English.
	Items int `json:"items"`
}

// LanguageList is the answer of ListLanguages.
type LanguageList struct {
	Version string `json:"version"`
	// Default is the language used when no lang is given ("en_us").
	Default   string     `json:"default"`
	Languages []Language `json:"languages"`
}

// Item is one item, named in the language asked for.
type Item struct {
	// ID is what the game's commands take, e.g. "minecraft:diamond_sword".
	ID string `json:"id"`
	// Name falls back to English, then to the id.
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Icon is the URL of the item's own image; nil for items drawn by code.
	Icon *string `json:"icon"`
	// Sprite is the icon's [column, row] in the version's atlas; nil without one.
	Sprite *[2]int `json:"sprite"`
	// MaxStack is how many fit in one slot; 0 when not known.
	MaxStack int `json:"max_stack,omitempty"`
	// MaxQuality is the highest quality the item is made at (e.g. Valheim); 0 otherwise.
	MaxQuality int `json:"max_quality,omitempty"`
	// Type is what the game itself says the item is ("material", "helmet"...), "" when unknown.
	Type string `json:"type,omitempty"`
}

// ItemPage is one page of ListItems.
type ItemPage struct {
	Version string `json:"version"`
	Lang    string `json:"lang"`
	// Total is how many items match the query in all pages.
	Total  int    `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	Items  []Item `json:"items"`
}

// SpriteRect is where an item's icon is inside the atlas, in pixels.
type SpriteRect struct {
	// URL is the atlas image.
	URL                 string
	X, Y, Width, Height int
}

// IconURL is the URL of the item's own icon, or "" without one.
func (it Item) IconURL() string {
	if it.Icon == nil {
		return ""
	}
	return *it.Icon
}

// SpriteRect is the pixel rectangle of the item's icon in atlas (from
// [Client.GetVersion]); ok is false when the item has no sprite or there is
// no atlas.
func (it Item) SpriteRect(atlas *Atlas) (r SpriteRect, ok bool) {
	if atlas == nil || it.Sprite == nil {
		return SpriteRect{}, false
	}
	col, row := it.Sprite[0], it.Sprite[1]
	return SpriteRect{URL: atlas.URL, X: col * atlas.Cell, Y: row * atlas.Cell, Width: atlas.Cell, Height: atlas.Cell}, true
}
