package datadog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

const GaugeMetricType string = "gauge"
const CounterMetricType string = "counter"

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
	BuildMetric(metricType string, metricName string, count float32, tags ...string) Metric
}

type client struct {
	apiKey string
	client *http.Client
}

func NewClient(apiKey string) *client {
	return &client{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 1 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}

func (c *client) BuildMetric(metricType string, metricName string, count float32, tags ...string) Metric {
	return Metric{
		Name: metricName,
		Type: metricType,
		Points: []Point{
			{Timestamp: time.Now(), Value: count},
		},
		Tags: tags,
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
		return fmt.Errorf("error sending metric to datadog: %s", err.Error())
	}

	if resp.StatusCode != http.StatusAccepted {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed reading body of error: %s", err.Error())
		}

		return fmt.Errorf("bad response (!202): %d - %s", resp.StatusCode, string(body))
	}

	return resp.Body.Close()
}
