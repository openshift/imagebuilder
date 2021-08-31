// +build conformance,!windows

package dockerclient

import (
	"os"
	"syscall"
)

type hardlinkCheckerKey struct {
	device, inode uint64
}

func (h *hardlinkChecker) makeHardlinkCheckerKey(info os.FileInfo) *hardlinkCheckerKey {
	sys := info.Sys()
	if stat, ok := sys.(*syscall.Stat_t); ok && (stat.Nlink > 1) {
		return &hardlinkCheckerKey{device: uint64(stat.Dev), inode: uint64(stat.Ino)}
	}
	return nil
}
