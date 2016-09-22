package revok

import (
	"encoding/json"
	"errors"
	"log"

	raven "github.com/getsentry/raven-go"

	"code.cloudfoundry.org/lager"
)

// Sink is the type that represents the sink that will emit errors to Sentry.
type Sink struct {
}

// NewSentrySink creates a new Sink for use with Lager.
func NewSentrySink(dsn, env string) *Sink {
	raven.SetDSN(dsn)
	raven.SetEnvironment(env)

	return &Sink{}
}

// Log will send any error log lines up to Sentry.
func (s *Sink) Log(line lager.LogFormat) {
	if line.LogLevel < lager.ERROR {
		return
	}

	if errStr, ok := line.Data["error"].(string); ok {
		delete(line.Data, "message")
		delete(line.Data, "error")

		tags := map[string]string{}
		for k, v := range line.Data {
			bs, err := json.Marshal(v)
			if err != nil {
				log.Printf("error marshaling JSON: %s", err)
				continue
			}

			tags[k] = string(bs)
		}

		e := raven.NewException(errors.New(errStr), raven.NewStacktrace(1, 3, []string{}))
		e.Type = line.Message
		raven.DefaultClient.Capture(raven.NewPacket(errStr, e), tags)
	}
}
