package trace

import cloudtrace "cloud.google.com/go/trace"

//go:generate counterfeiter . Client

type Client interface {
	NewSpan(name string) *cloudtrace.Span
}
