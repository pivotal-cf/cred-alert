package net

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"time"

	"code.cloudfoundry.org/clock"
	"github.com/cenkalti/backoff"
)

type retryingClient struct {
	client Client
	clock  clock.Clock
}

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

	var resp *http.Response

	makeRequest := func() error {
		req, err := http.NewRequest(orgReq.Method, orgReq.URL.String(), bytes.NewBuffer(body))
		if err != nil {
			return err
		}

		req.Header = orgReq.Header

		resp, err = c.client.Do(req)
		if err != nil {
			return err
		}

		return nil
	}

	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = time.Minute
	bo.Clock = c.clock

	if err := c.retry(makeRequest, bo); err != nil {
		return nil, fmt.Errorf("request failed after retry: %s", err.Error())
	}

	return resp, nil
}

// This function is mainly a copy of the retry function from the `backoff` package
// but modified to use our clock interface.
func (c *retryingClient) retry(operation backoff.Operation, b backoff.BackOff) error {
	var err error
	var next time.Duration

	b.Reset()

	for {
		if err = operation(); err == nil {
			return nil
		}

		if next = b.NextBackOff(); next == backoff.Stop {
			return err
		}

		c.clock.Sleep(next)
	}
}
