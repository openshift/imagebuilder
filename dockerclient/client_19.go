// +build !go1.10

package dockerclient

import "archive/tar"

func forceHeaderFormat(h *tar.Header) error {
	return nil
}
