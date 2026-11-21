/**
 * Models of the LobbyTick game-data API responses. Field names are exactly
 * the ones the API sends (snake_case), so a response can be logged, cached or
 * re-served without conversion.
 */

/** Game ids LobbyTick knows today. Other strings may appear as games are added. */
export type GameId = "minecraft" | "valheim" | "zomboid" | (string & {});

/**
 * How an item's icon was made, which is also roughly what the item is.
 * - `flat`: a 2D icon (tools, food, ingots)
 * - `block`: a block drawn in 3D
 * - `special`: drawn by the game's own code; it has no icon yet
 * - `empty`: not something a player can hold (never listed by `listItems`)
 */
export type ItemKind = "flat" | "block" | "special" | "empty" | (string & {});

/** One version of a game, as listed in a {@link Game}. */
export interface VersionSummary {
  version: string;
  protocol: number;
  /** How many items the catalog has (including `empty` ones). */
  items: number;
  /** How many languages item names exist in. */
  languages: number;
}

export interface Game {
  id: GameId;
  name: string;
  /** Newest first. Empty for a game LobbyTick has no data for yet. */
  versions: VersionSummary[];
}

export interface GameList {
  games: Game[];
}

/** Every icon of a version in one image: a grid of square cells. */
export interface Atlas {
  url: string;
  columns: number;
  rows: number;
  /** Side of each cell, in pixels. */
  cell: number;
}

export interface Version {
  version: string;
  protocol: number;
  /** 0 when the game has no such thing. */
  world_version: number;
  items: number;
  languages: number;
  /** Side of each icon, in pixels. */
  icon_size: number;
  atlas: Atlas | null;
}

export interface Language {
  /** The game's own code, e.g. `pt_br`. */
  code: string;
  name: string;
  region: string;
  /** How many items it names; the rest fall back to English. */
  items: number;
}

export interface LanguageList {
  version: string;
  /** The language used when no `lang` is given (`en_us`). */
  default: string;
  languages: Language[];
}

export interface Item {
  /** What the game's commands take, e.g. `minecraft:diamond_sword`. */
  id: string;
  /** Name in the requested language (falls back to English, then to the id). */
  name: string;
  kind: ItemKind;
  /** URL of the item's own image, or null for items the game draws by code. */
  icon: string | null;
  /** `[column, row]` of the icon in the version's atlas, or null. */
  sprite: [number, number] | null;
  /** How many fit in one slot; absent when not known. */
  max_stack?: number;
  /** Highest quality the item is made at (games that have quality, e.g. Valheim). */
  max_quality?: number;
  /** What the game itself says the item is ("material", "helmet"...), when known. */
  type?: string;
}

export interface ItemPage {
  version: string;
  lang: string;
  /** How many items match the query in total. */
  total: number;
  offset: number;
  limit: number;
  items: Item[];
}
