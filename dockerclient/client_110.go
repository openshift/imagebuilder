// +build go1.10

package dockerclient

import "archive/tar"

func forceHeaderFormat(h *tar.Header) error {
	if (h.Uid > 0x1fffff || h.Gid > 0x1fffff) && h.Format == tar.FormatUSTAR {
		h.Format = tar.FormatPAX
	}
	return nil
}
