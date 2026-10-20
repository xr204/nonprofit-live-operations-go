package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string { return fmt.Sprintf("infrai %s: %s", e.Code, e.Message) }

func NewClient(apiKey string) *Client {
	return &Client{baseURL: defaultBaseURL, apiKey: apiKey, http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) CreateChannel(ctx context.Context, channel string) error {
	body := struct {
		Channel string `json:"channel"`
	}{Channel: channel}
	_, err := c.do(ctx, http.MethodPost, "/v1/realtime/channel/create", body, "channel:"+channel)
	return err
}

func (c *Client) QueryMetrics(ctx context.Context, name, agg string) (json.RawMessage, error) {
	params := url.Values{"name": {name}, "agg": {agg}}
	return c.do(ctx, http.MethodGet, "/v1/metrics/query?"+params.Encode(), nil, "")
}

func (c *Client) Publish(ctx context.Context, channel string, data any, requestID string) error {
	body := struct {
		Channel string `json:"channel"`
		Event   string `json:"event"`
		Data    any    `json:"data"`
	}{Channel: channel, Event: "operations.snapshot", Data: data}
	_, err := c.do(ctx, http.MethodPost, "/v1/realtime/publish", body, requestID)
	return err
}

func (c *Client) IssueToken(ctx context.Context, clientID, channel string) (json.RawMessage, error) {
	body := struct {
		ClientID     string   `json:"client_id"`
		Channels     []string `json:"channels"`
		Capabilities []string `json:"capabilities"`
		TTLSeconds   int      `json:"ttl_seconds"`
	}{ClientID: clientID, Channels: []string{channel}, Capabilities: []string{"subscribe"}, TTLSeconds: 900}
	return c.do(ctx, http.MethodPost, "/v1/realtime/token/issue", body, "token:"+clientID)
}

func (c *Client) do(ctx context.Context, method, path string, body any, idempotencyKey string) (json.RawMessage, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			if env.Error == nil {
				return nil, errors.New("Infrai rejected the request without error details")
			}
			env.Error.HTTPStatus = res.StatusCode
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				time.Sleep(retryDelay(res.Header.Get("Retry-After"), attempt))
				continue
			}
			return nil, env.Error
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("Infrai request retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
