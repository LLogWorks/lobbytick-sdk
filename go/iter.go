package lobbytick

import (
	"context"
	"fmt"
	"iter"
)

// IterItemsParams filter IterItems.
type IterItemsParams struct {
	Lang string
	Q    string
	Kind string
	// PageSize is items per request, 1..5000. Default 1000.
	PageSize int
}

// IterItems yields every matching item, fetching pages as needed. On an error
// it yields (zero Item, err) once and stops. With version "latest", the
// version of the first page is used for the rest, so a deploy mid-way cannot
// mix two catalogs.
//
//	for it, err := range c.IterItems(ctx, "valheim", "latest", lobbytick.IterItemsParams{}) {
//		if err != nil { return err }
//		fmt.Println(it.ID, it.Name)
//	}
func (c *Client) IterItems(ctx context.Context, game, version string, p IterItemsParams) iter.Seq2[Item, error] {
	return func(yield func(Item, error) bool) {
		size := p.PageSize
		if size == 0 {
			size = 1000
		}
		if size < 1 || size > MaxPageSize {
			yield(Item{}, fmt.Errorf("lobbytick: PageSize must be between 1 and %d", MaxPageSize))
			return
		}
		offset := 0
		for {
			page, err := c.ListItems(ctx, game, version, ListItemsParams{Lang: p.Lang, Q: p.Q, Kind: p.Kind, Limit: size, Offset: offset})
			if err != nil {
				yield(Item{}, err)
				return
			}
			version = page.Version
			for _, it := range page.Items {
				if !yield(it, nil) {
					return
				}
			}
			offset += len(page.Items)
			if len(page.Items) == 0 || offset >= page.Total {
				return
			}
		}
	}
}
