package net

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type retryingClient struct {
	client Client
}

const maxRetries = 3

var delays = [3][2]int{
	{250, 750},
	{375, 1125},
	{562, 1687},
}

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

	req, _ := http.NewRequest(orgReq.Method, orgReq.URL.String(), body)
	req.Header = orgReq.Header
	if resp, err = c.client.Do(req); err == nil {
		return resp, nil
	}

	for i := 0; i < maxRetries; i++ {
		req, _ := http.NewRequest(orgReq.Method, orgReq.URL.String(), body)
		req.Header = orgReq.Header

		random := rand.Intn(delays[i][1]-delays[i][0]) + delays[i][0]
		time.Sleep(time.Duration(random) * time.Millisecond)
		resp, err = c.client.Do(req)

		if err == nil {
			break
		}
	}

	return resp, err
}
