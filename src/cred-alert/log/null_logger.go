package log

import "code.cloudfoundry.org/lager"

type NullLogger struct{}

func (l *NullLogger) RegisterSink(lager.Sink)                              {}
func (l *NullLogger) Session(task string, data ...lager.Data) lager.Logger { return l }
func (l *NullLogger) SessionName() string                                  { return "" }
func (l *NullLogger) Debug(action string, data ...lager.Data)              {}
func (l *NullLogger) Info(action string, data ...lager.Data)               {}
func (l *NullLogger) Error(action string, err error, data ...lager.Data)   {}
func (l *NullLogger) Fatal(action string, err error, data ...lager.Data)   {}
func (l *NullLogger) WithData(lager.Data) lager.Logger                     { return l }

func NewNullLogger() *NullLogger {
	return &NullLogger{}
}
