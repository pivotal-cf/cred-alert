package net

import (
	"bytes"
	"io"
	"net/http"
)

type retryingClient struct {
	client Client
}

const maxRetries = 4

func NewRetryingClient(c Client) Client {
	return &retryingClient{
		client: c,
	}
}

func (c *retryingClient) Do(orgReq *http.Request) (*http.Response, error) {
	var (
		err  error
		resp *http.Response
		body io.Reader
	)

	if orgReq.Body != nil {
		buf := bytes.NewBuffer([]byte{})
		buf.ReadFrom(orgReq.Body)
		body = buf
	}

	for i := 0; i < maxRetries; i++ {
		req, _ := http.NewRequest(orgReq.Method, orgReq.URL.String(), body)
		req.Header = orgReq.Header

		resp, err = c.client.Do(req)

		if err == nil {
			break
		}
	}

	return resp, err
}
