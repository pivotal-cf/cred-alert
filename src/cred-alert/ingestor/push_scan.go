package ingestor

type PushScan struct {
	Owner      string
	Repository string
	From       string
	To         string
	Private    bool
}
