package libmangal

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"net/http"
	"runtime"

	"github.com/gen2brain/webp"
	"github.com/luevano/libmangal/mangadata"
	"golang.org/x/sync/errgroup"
)

// DefaultWebPQuality is the WebP quality used when DownloadOptions.WebPQuality is 0.
const DefaultWebPQuality = 80

// webpPage is a page converted to WebP.
type webpPage struct {
	mangadata.PageWithImage
}

func (webpPage) Extension() string { return ".webp" }

// convertToWebP encodes the pages as lossy WebP, GOMAXPROCS at a time. Pages that already
// are WebP stay as they are, so do the ones that can't be decoded (logged).
func (c *Client) convertToWebP(ctx context.Context, pages []mangadata.PageWithImage, quality int) error {
	if quality == 0 {
		quality = DefaultWebPQuality
	}
	c.logger.Log("converting %d pages to WebP (quality %d)", len(pages), quality)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.GOMAXPROCS(0))
	for i, page := range pages {
		g.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			data := page.Image()
			if http.DetectContentType(data) == "image/webp" {
				pages[i] = webpPage{page}
				return nil
			}
			img, format, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				c.logger.Log("page #%03d: not converted to WebP: %s", i+1, err)
				return nil
			}
			var buf bytes.Buffer
			if err := webp.Encode(&buf, img, webp.Options{Quality: quality, Method: webp.DefaultMethod}); err != nil {
				return fmt.Errorf("page %d: encode %s as WebP: %w", i+1, format, err)
			}
			page.SetImage(buf.Bytes())
			pages[i] = webpPage{page}
			return nil
		})
	}
	return g.Wait()
}
