package metrics

import "code.cloudfoundry.org/lager"

//go:generate counterfeiter . Gauge

type Gauge interface {
	Update(lager.Logger, float32, ...string)
}
