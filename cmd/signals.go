//go:build !linux

package main

import (
	"os"
	"syscall"
)

var refreshSignals []os.Signal
var signals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
