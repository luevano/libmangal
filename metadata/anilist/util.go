package anilist

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/luevano/libmangal/metadata/internal/ratelimit"
)

type apiRequestBody struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type apiResponse[Data any] struct {
	Errors []struct {
		Message string `json:"message"`
		Status  int    `json:"status"`
	} `json:"errors"`
	Data Data `json:"data"`
}

// maxErrorBody is how much of an error response is read.
const maxErrorBody = 1 << 16

func sendRequest[Data any](
	ctx context.Context,
	anilist *Anilist,
	requestBody apiRequestBody,
) (data Data, err error) {
	err = ratelimit.Retry(ctx, anilist.logger, info.ID, "AniList", func() error {
		data, err = sendRequestOnce[Data](ctx, anilist, requestBody)
		return err
	})
	return data, err
}

func sendRequestOnce[Data any](
	ctx context.Context,
	anilist *Anilist,
	requestBody apiRequestBody,
) (data Data, err error) {
	marshalled, err := json.Marshal(requestBody)
	if err != nil {
		return data, fmt.Errorf("anilist: encode request: %w", err)
	}

	resp, err := anilist.genericRequest(ctx, http.MethodPost, anilist.apiURL, bytes.NewReader(marshalled), true)
	if err != nil {
		return data, fmt.Errorf("anilist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return data, &RateLimitError{RetryAfter: retryAfter(resp.Header)}
	}

	// errors come with a body like {"errors": [{"message": "Not Found.", "status": 404}]}
	var res apiResponse[Data]
	body := io.Reader(resp.Body)
	if resp.StatusCode != http.StatusOK {
		body = io.LimitReader(body, maxErrorBody)
	}
	decodeErr := json.NewDecoder(body).Decode(&res)
	if resp.StatusCode != http.StatusOK || len(res.Errors) != 0 {
		httpErr := &HTTPError{StatusCode: resp.StatusCode}
		if len(res.Errors) != 0 {
			httpErr.Message = res.Errors[0].Message
			if status := res.Errors[0].Status; status != 0 {
				httpErr.StatusCode = status
			}
		}
		return data, httpErr
	}
	if decodeErr != nil {
		return data, fmt.Errorf("anilist: decode response: %w", decodeErr)
	}

	return res.Data, nil
}

func (p *Anilist) genericRequest(ctx context.Context, method, url string, body io.Reader, authIfAvailable bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", cmp.Or(p.options.UserAgent, DefaultUserAgent))

	if token := p.session.Token(); authIfAvailable && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return p.options.HTTPClient.Do(req)
}
