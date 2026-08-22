//go:build !linux

package data

import "os"

// preserveOwnership stub for OS that does not support ownerships
func preserveOwnership(_ os.FileInfo, _ *os.File) error {
	return nil
}
