package ingestor

import "time"

type PushScan struct {
	Owner      string
	Repository string
	PushTime   time.Time
}
