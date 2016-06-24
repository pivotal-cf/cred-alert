package datadog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var APIURL = "https://app.datadoghq.com"

type Series []Metric

type Metric struct {
	Name   string   `json:"metric"`
	Points []Point  `json:"points"`
	Type   string   `json:"type"`
	Host   string   `json:"host"`
	Tags   []string `json:"tags"`
}

type Point struct {
	Timestamp time.Time
	Value     float32
}

func (p Point) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`[%d, %f]`, p.Timestamp.Unix(), p.Value)), nil
}

func (p *Point) UnmarshalJSON(data []byte) error {
	var tuple []float64

	if err := json.Unmarshal(data, &tuple); err != nil {
		return err
	}

	p.Timestamp = time.Unix(int64(tuple[0]), 0)
	p.Value = float32(tuple[1])

	return nil
}

type request struct {
	Series Series `json:"series"`
}

//go:generate counterfeiter . Client

type Client interface {
	PublishSeries(series Series) error
}

type client struct {
	apiKey string
	client *http.Client
}

func NewClient(apiKey string) *client {
	return &client{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

func (c *client) PublishSeries(series Series) error {
	request := request{
		Series: series,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", APIURL+"/api/v1/series", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("building request: %s", err)
	}

	auth := url.Values{}
	auth.Set("api_key", c.apiKey)
	req.URL.RawQuery = auth.Encode()

	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Content-length", strconv.Itoa(len(payload)))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("response: %s", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("bad response (!202): %s\n", err)
	}

	return resp.Body.Close()
}
