package utils

import (
	"fmt"
	"net/http"
	"time"
)

// RetryableHTTPClient implements a HTTP client with retry logic
type RetryableHTTPClient struct {
	client  *http.Client
	retries int
	backoff time.Duration
}

func NewRetryableHTTPClient(retries int, timeout time.Duration, backoff time.Duration) *RetryableHTTPClient {
	return &RetryableHTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		retries: retries,
		backoff: backoff,
	}
}

func (c *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error
	for i := 0; i <= c.retries; i++ {
		resp, err := c.client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("server returned status %d", resp.StatusCode)
			resp.Body.Close()
		}

		if i < c.retries {
			time.Sleep(c.backoff * time.Duration(i+1))
		}
	}

	return nil, fmt.Errorf("after %d retries, last error: %s", c.retries, lastErr)
}
