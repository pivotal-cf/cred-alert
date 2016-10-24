package net

import "net/http"

//go:generate counterfeiter . Client

type Client interface {
	Do(req *http.Request) (*http.Response, error)
}
