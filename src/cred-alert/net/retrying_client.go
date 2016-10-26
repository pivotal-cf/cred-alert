package net

import (
	"bytes"
	"errors"
	"io/ioutil"
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
	body, err := ioutil.ReadAll(orgReq.Body)
	if err != nil {
		return nil, err
	}

	req, reqErr := http.NewRequest(orgReq.Method, orgReq.URL.String(), bytes.NewBuffer(body))
	if reqErr != nil {
		return nil, reqErr
	}

	req.Header = orgReq.Header
	if resp, err := c.client.Do(req); err == nil {
		return resp, nil
	}

	for i := 0; i < maxRetries; i++ {
		req, reqErr := http.NewRequest(orgReq.Method, orgReq.URL.String(), bytes.NewBuffer(body))
		if reqErr != nil {
			return nil, reqErr
		}

		req.Header = orgReq.Header

		random := rand.Intn(delays[i][1]-delays[i][0]) + delays[i][0]
		time.Sleep(time.Duration(random) * time.Millisecond)
		resp, err := c.client.Do(req)
		if err != nil {
			continue
		}

		return resp, nil
	}

	return nil, errors.New("request failed after retry")
}
