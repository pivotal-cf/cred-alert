package kolsch

import "code.cloudfoundry.org/lager"

type nullLogger struct{}

func (l *nullLogger) RegisterSink(lager.Sink)                              {}
func (l *nullLogger) Session(task string, data ...lager.Data) lager.Logger { return l }
func (l *nullLogger) SessionName() string                                  { return "" }
func (l *nullLogger) Debug(action string, data ...lager.Data)              {}
func (l *nullLogger) Info(action string, data ...lager.Data)               {}
func (l *nullLogger) Error(action string, err error, data ...lager.Data)   {}
func (l *nullLogger) Fatal(action string, err error, data ...lager.Data)   {}
func (l *nullLogger) WithData(lager.Data) lager.Logger                     { return l }

func NewLogger() *nullLogger {
	return &nullLogger{}
}
