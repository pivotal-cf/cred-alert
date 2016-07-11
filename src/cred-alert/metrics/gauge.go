package metrics

import "github.com/pivotal-golang/lager"

//go:generate counterfeiter . Gauge

type Gauge interface {
	Update(lager.Logger, float32, ...string)
}
