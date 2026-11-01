package libmangal

import (
	"bytes"
	"context"
	"image"
	"net/http"
	"runtime"

	"github.com/gen2brain/webp"
	"github.com/luevano/libmangal/mangadata"
	"golang.org/x/sync/errgroup"
)

// DefaultImageQuality is the quality used when DownloadOptions.ImageQuality is 0: on real
// PNG pages q90 has SSIM .9960 at 67% of the size, q80 .9888 at 49%.
const DefaultImageQuality = 90

// webpPage is a page converted to WebP.
type webpPage struct {
	mangadata.PageWithImage
}

func (webpPage) Extension() string { return ".webp" }

// convertToWebP encodes the pages as lossy WebP, GOMAXPROCS at a time. A page keeps its
// original bytes when it already is WebP (decoders return limited-range YCbCr, encoding that
// again washes out the blacks), when it can't be decoded or encoded (logged: libwebp stops at
// 16383 px a side), or when the WebP wouldn't be smaller (usual for JPEGs).
func (c *Client) convertToWebP(ctx context.Context, pages []mangadata.PageWithImage, quality int) error {
	if quality == 0 {
		quality = DefaultImageQuality
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
				c.logger.Log("page #%03d: not converted to WebP: encode %s: %s", i+1, format, err)
				return nil
			}
			if buf.Len() >= len(data) {
				c.logger.Log("page #%03d: kept as %s: WebP isn't smaller (%d >= %d bytes)", i+1, format, buf.Len(), len(data))
				return nil
			}
			page.SetImage(buf.Bytes())
			pages[i] = webpPage{page}
			return nil
		})
	}
	return g.Wait()
}
