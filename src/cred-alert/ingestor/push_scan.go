package ingestor

import "time"

type PushScan struct {
	Owner      string
	Repository string
	From       string
	To         string
	Private    bool
	PushTime   time.Time
}
