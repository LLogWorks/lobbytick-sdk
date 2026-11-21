import type { Atlas, Item } from "./types.js";

/** Where an item's icon is inside a version's atlas, in pixels. */
export interface SpriteRect {
  /** The atlas image URL. */
  url: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

/** The URL of an item's own icon image, or null for items without one. */
export function iconUrl(item: Pick<Item, "icon">): string | null {
  return item.icon ?? null;
}

/**
 * The pixel rectangle of an item's icon in the atlas (from `getVersion(...).atlas`),
 * or null when the item has no sprite or the version has no atlas.
 *
 * In CSS: `background: url(${r.url}) -${r.x}px -${r.y}px; width: ${r.width}px; height: ${r.height}px`.
 */
export function spriteRect(item: Pick<Item, "sprite">, atlas: Atlas | null | undefined): SpriteRect | null {
  if (!atlas || !item.sprite) return null;
  const [column, row] = item.sprite;
  return { url: atlas.url, x: column * atlas.cell, y: row * atlas.cell, width: atlas.cell, height: atlas.cell };
}
