// Runs against the real API only when LOBBYTICK_LIVE=1 (a handful of requests).
import { describe, expect, it } from "vitest";
import { LobbyTick, NotFoundError, spriteRect } from "../src/index.js";

const live = process.env.LOBBYTICK_LIVE === "1";

describe.skipIf(!live)("live API", () => {
  const c = new LobbyTick({ baseUrl: process.env.LOBBYTICK_BASE_URL, maxRetries: 2 });

  it("reads games, a version, languages, items and one item", async () => {
    const games = await c.listGames();
    expect(games.map((g) => g.id)).toContain("minecraft");

    const v = await c.getVersion("minecraft", "latest");
    expect(v.atlas?.url).toMatch(/^https:\/\/assets\.lobbytick\.com\//);

    const langs = await c.listLanguages("valheim", "latest");
    expect(langs.default).toBe("en_us");
    expect(langs.languages.some((l) => l.code === "pt_br")).toBe(true);

    const page = await c.listItems("minecraft", v.version, { lang: "pt_br", q: "espada diamante", limit: 5 });
    expect(page.items[0]?.id).toBe("minecraft:diamond_sword");

    const item = await c.getItem("minecraft", v.version, "minecraft:diamond_sword", { lang: "pt_br" });
    expect(item.name).toBe("Espada de Diamante");
    expect(spriteRect(item, v.atlas)?.width).toBe(v.atlas?.cell);

    let n = 0;
    for await (const _ of c.iterItems("valheim", "latest", { pageSize: 5000 })) n++;
    expect(n).toBeGreaterThan(500);

    await expect(c.getGame("nope")).rejects.toBeInstanceOf(NotFoundError);
  }, 30_000);
});
