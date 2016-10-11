package revok

import (
	"fmt"
	"net/http"
)

// TODO: Actually test dependencies.

func ObliviousHealthCheck() http.Handler {
	return &healthCheck{}
}

type healthCheck struct{}

func (hc *healthCheck) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ok")
}
