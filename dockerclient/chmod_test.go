package dockerclient

import (
	"archive/tar"
	"testing"

	"github.com/tonistiigi/dchapes-mode"
)

// TestApplyChmod covers the tar-header rewrite done for --chmod in
// CopyContainer, which the conformance suite only exercises behind a build
// tag and a live daemon.
func TestApplyChmod(t *testing.T) {
	tests := []struct {
		name     string
		chmod    string
		typeflag byte
		inMode   int64
		wantMode int64
	}{
		// Numeric modes replace the permission bits (and any special bits)
		// of a regular file, keeping the file-type bits.
		{name: "numeric basic", chmod: "755", typeflag: tar.TypeReg, inMode: 0o100644, wantMode: 0o100755},
		{name: "numeric clears exec", chmod: "644", typeflag: tar.TypeReg, inMode: 0o100755, wantMode: 0o100644},
		{name: "numeric setuid", chmod: "4755", typeflag: tar.TypeReg, inMode: 0o100644, wantMode: 0o104755},
		{name: "numeric setgid", chmod: "2755", typeflag: tar.TypeReg, inMode: 0o100644, wantMode: 0o102755},
		{name: "numeric sticky", chmod: "1755", typeflag: tar.TypeReg, inMode: 0o100644, wantMode: 0o101755},
		// Numeric modes are absolute: a pre-existing setuid bit is cleared,
		// matching chmod(1).
		{name: "numeric clears setuid", chmod: "755", typeflag: tar.TypeReg, inMode: 0o104755, wantMode: 0o100755},
		// Symbolic modes.
		{name: "symbolic add exec", chmod: "+x", typeflag: tar.TypeReg, inMode: 0o100644, wantMode: 0o100755},
		{name: "symbolic clause list", chmod: "u+x,go-w", typeflag: tar.TypeReg, inMode: 0o100644, wantMode: 0o100744},
		{name: "symbolic multi group", chmod: "u+rX-w", typeflag: tar.TypeReg, inMode: 0o100700, wantMode: 0o100500},
		{name: "symbolic setuid", chmod: "u+s", typeflag: tar.TypeReg, inMode: 0o100644, wantMode: 0o104644},
		{name: "symbolic sticky", chmod: "+t", typeflag: tar.TypeReg, inMode: 0o100755, wantMode: 0o101755},
		// The conditional X: granted on directories, and on regular files
		// only if an execute bit is already set.
		{name: "X on directory", chmod: "u=rwX,go=rX", typeflag: tar.TypeDir, inMode: 0o040644, wantMode: 0o040755},
		{name: "X on non-exec regular", chmod: "u=rwX", typeflag: tar.TypeReg, inMode: 0o100644, wantMode: 0o100644},
		{name: "X on exec regular", chmod: "a+rX", typeflag: tar.TypeReg, inMode: 0o100700, wantMode: 0o100755},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			set, err := mode.Parse(tc.chmod)
			if err != nil {
				t.Fatalf("parsing %q: %v", tc.chmod, err)
			}
			h := &tar.Header{Typeflag: tc.typeflag, Mode: tc.inMode}
			applyChmod(h, set)
			if h.Mode != tc.wantMode {
				t.Errorf("chmod %q on header %#o (%c): got %#o, want %#o",
					tc.chmod, tc.inMode, tc.typeflag, h.Mode, tc.wantMode)
			}
		})
	}
}
