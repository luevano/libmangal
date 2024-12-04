package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
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

func sendRequest[Data any](
	ctx context.Context,
	anilist *Anilist,
	requestBody apiRequestBody,
) (data Data, err error) {
	marshalled, err := json.Marshal(requestBody)
	if err != nil {
		return data, err
	}

	resp, err := anilist.genericRequest(ctx, http.MethodPost, apiURL, bytes.NewReader(marshalled), true)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()

	// https://anilist.gitbook.io/anilist-apiv2-docs/docs/guide/rate-limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter == "" {
			// 90 seconds
			retryAfter = "90"
		}

		seconds, err := strconv.Atoi(retryAfter)
		if err != nil {
			return data, err
		}

		anilist.logger.Log("rate limited, retrying in %d seconds", seconds)

		select {
		case <-time.After(time.Duration(seconds) * time.Second):
		case <-ctx.Done():
			return data, ctx.Err()
		}

		return sendRequest[Data](ctx, anilist, requestBody)
	}

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf(resp.Status)
	}

	var res apiResponse[Data]
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return data, err
	}

	if res.Errors != nil {
		err := res.Errors[0]
		return data, errors.New(err.Message)
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

	if authIfAvailable && p.Authenticated() {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}

	return p.options.HTTPClient.Do(req)
}
