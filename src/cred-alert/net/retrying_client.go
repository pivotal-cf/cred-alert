package net

import "net/http"

type retryingClient struct {
	retryingClient Client
}

const maxRetries = 4

func NewRetryingClient(c Client) Client {
	return &retryingClient{
		retryingClient: c,
	}
}

func (c *retryingClient) Do(req *http.Request) (*http.Response, error) {
	var (
		err  error
		resp *http.Response
	)

	for i := 0; i < maxRetries; i++ {
		resp, err = c.retryingClient.Do(req)

		if err == nil {
			return resp, nil
		}
	}

	return nil, err
}
