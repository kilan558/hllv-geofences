package rconv2

import (
	"errors"
	"syscall"
	"testing"
)

func TestIsBrokenHllConnectionTreatsExhaustedReconnectsAsBroken(t *testing.T) {
	if !IsBrokenHllConnection(ReconnectTriesExceeded) {
		t.Fatal("ReconnectTriesExceeded should retire the socket")
	}
	if !IsBrokenHllConnection(syscall.EPIPE) {
		t.Fatal("EPIPE should retire the socket")
	}
	if IsBrokenHllConnection(errors.New("unexpected status 500")) {
		t.Fatal("application errors should not retire the socket")
	}
	if IsBrokenHllConnection(nil) {
		t.Fatal("nil should not be treated as a broken connection")
	}
}
