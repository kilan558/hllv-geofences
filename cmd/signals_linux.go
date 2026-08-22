package main

import (
	"os"
	"syscall"
)

var refreshSignals = []os.Signal{syscall.SIGUSR1}
var signals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1}
