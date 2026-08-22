package data

import (
	"fmt"
	"os"
	"syscall"
)

// preserveOwnership implementation for the linux filesystem. Extracted from config.go to allow compilation on
// non-linux OS.
func preserveOwnership(orig os.FileInfo, f *os.File) error {
	if owner, ok := orig.Sys().(*syscall.Stat_t); ok &&
		(owner.Uid != uint32(os.Geteuid()) || owner.Gid != uint32(os.Getegid())) {
		if err := f.Chown(int(owner.Uid), int(owner.Gid)); err != nil {
			return fmt.Errorf("preserve config ownership: %w", err)
		}
	}

	return nil
}
