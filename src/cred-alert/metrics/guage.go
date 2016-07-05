package metrics

import "github.com/pivotal-golang/lager"

//go:generate counterfeiter . Guage

type Guage interface {
	Update(lager.Logger, float32)
}
