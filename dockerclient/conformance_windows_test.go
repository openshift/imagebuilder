// +build conformance,windows

package dockerclient

import (
	"os"
)

type hardlinkCheckerKey struct {
}

func (h *hardlinkChecker) makeHardlinkCheckerKey(info os.FileInfo) *hardlinkCheckerKey {
	return nil
}
