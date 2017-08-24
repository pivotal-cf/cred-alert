package revok

import (
	"fmt"
	"net/http"
)

// TODO: Actually test dependencies.

func NewObliviousHealthCheck() *ObliviousHealthCheck {
	return &ObliviousHealthCheck{}
}

type ObliviousHealthCheck struct{}

func (hc *ObliviousHealthCheck) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ok")
}
