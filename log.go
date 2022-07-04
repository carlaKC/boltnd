package boltnd

import "github.com/btcsuite/btclog"

// LogRegistration is the function signature used to register loggers.
type LogRegistration func(logger btclog.Logger)

// Subsystem defines the logging code for this subsystem.
const Subsystem = "B12-OFRS"

// log is a logger that is initialized with no output filters.  This
// means the package will not perform any logging by default until the caller
// requests it.
var log = btclog.Disabled

// DisableLog disables all library log output.  Logging output is disabled
// by default until UseLogger is called.
func DisableLog() {
	UseLogger(btclog.Disabled)
}

// UseLogger uses a specified Logger to output package logging info.
// This should be used in preference to SetLogWriter if the caller is also
// using btclog.
func UseLogger(logger btclog.Logger) {
	log = logger
}
