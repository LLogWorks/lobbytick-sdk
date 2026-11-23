# lobbytick (Go)

Go client for the [LobbyTick](https://lobbytick.com) public game-data API: items of
Minecraft, Valheim and Project Zomboid per version, their names in every language and
their icons. Standard library only, context-aware, Go 1.23+ (`iter.Seq2` iterator).

```sh
go get github.com/LLogWorks/lobbytick-sdk/go
```

## Quick start

```go
import lobbytick "github.com/LLogWorks/lobbytick-sdk/go"

c := lobbytick.New(nil) // or lobbytick.New(&lobbytick.Options{MaxRetries: 3, Timeout: 10 * time.Second})
ctx := context.Background()

games, err := c.ListGames(ctx)
v, err := c.GetVersion(ctx, "minecraft", "latest")
langs, err := c.ListLanguages(ctx, "minecraft", v.Version)

page, err := c.ListItems(ctx, "minecraft", "latest", lobbytick.ListItemsParams{Lang: "pt_br", Q: "espada", Limit: 20})

for item, err := range c.IterItems(ctx, "zomboid", "latest", lobbytick.IterItemsParams{Kind: lobbytick.KindFlat}) {
	if err != nil {
		return err
	}
	fmt.Println(item.ID, item.Name)
}

sword, err := c.GetItem(ctx, "minecraft", "latest", "minecraft:diamond_sword", "ja_jp")
if r, ok := sword.SpriteRect(v.Atlas); ok {
	fmt.Println(r.URL, r.X, r.Y, r.Width, r.Height)
}

_, err = c.GetGame(ctx, "nope")
var apiErr *lobbytick.APIError
switch {
case errors.Is(err, lobbytick.ErrNotFound):
	// 404
case errors.Is(err, lobbytick.ErrRateLimited) && errors.As(err, &apiErr):
	time.Sleep(apiErr.RetryAfter)
}
```

## Options

| Field           | Default                       | Notes                                        |
| --------------- | ----------------------------- | -------------------------------------------- |
| `BaseURL`       | `https://api.lobbytick.com`   |                                              |
| `HTTPClient`    | `&http.Client{Timeout: 30s}`  | bring your own transport/proxy               |
| `Timeout`       | 30 s                          | only without `HTTPClient`; negative disables |
| `MaxRetries`    | 0                             | retries on 429/502/503/504                   |
| `MaxRetryDelay` | 60 s                          | never sleeps longer; returns the error       |
| `UserAgent`     | `lobbytick-sdk-go/0.1.0`      |                                              |

`*APIError` has `StatusCode`, `Message` (the API's own text), `Body`, `Header`,
`RetryAfter` and `RateLimit`. `""` as a version means `"latest"`.

## Data license

The code is MIT. Item names and images belong to Mojang/Microsoft, Iron Gate and The Indie
Stone and are not covered by the MIT license.
