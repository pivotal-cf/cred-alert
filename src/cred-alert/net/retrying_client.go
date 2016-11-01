package net

import (
	"bytes"
	"errors"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"code.cloudfoundry.org/clock"
)

type retryingClient struct {
	client Client
	clock  clock.Clock
}

const maxRetries = 3

func NewRetryingClient(c Client, clock clock.Clock) Client {
	return &retryingClient{
		client: c,
		clock:  clock,
	}
}

func (c *retryingClient) Do(orgReq *http.Request) (*http.Response, error) {
	body, err := ioutil.ReadAll(orgReq.Body)
	if err != nil {
		return nil, err
	}

	for i := 0; i < maxRetries+1; i++ {
		req, reqErr := http.NewRequest(orgReq.Method, orgReq.URL.String(), bytes.NewBuffer(body))
		if reqErr != nil {
			return nil, reqErr
		}

		req.Header = orgReq.Header

		if i != 0 {
			random := rand.Intn(delays[i-1][1]-delays[i-1][0]) + delays[i-1][0]
			c.clock.Sleep(time.Duration(random) * time.Millisecond)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			continue
		}

		return resp, nil
	}

	return nil, errors.New("request failed after retry")
}

var delays = [3][2]int{
	{250, 750},
	{375, 1125},
	{562, 1687},
}
