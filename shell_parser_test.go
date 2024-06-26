package imagebuilder

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessWord(t *testing.T) {
	envs := []string{
		"EDITOR=vim",
		"FOUREYES=iiii",
		"XMODIFIERS=@im=ibus",
	}
	testCases := []struct {
		pattern, expected string
		expectError       bool
	}{
		{"A", "A", false},
		{"${EDITOR}", "vim", false},
		{"${EDITOR:+emacs}", "emacs", false},
		{"${EDITOR:-emacs}", "vim", false},
		{"${EDITOR:-${FOUREYES}}", "vim", false},
		{"${EDITOR:+${FOUREYES}}", "iiii", false},
		{"${XMODIFIERS#*i}", "m=ibus", false},
		{"${XMODIFIERS##*i}", "bus", false},
		{"${XMODIFIERS#i*}", "@im=ibus", false},
		{"${XMODIFIERS##i*}", "@im=ibus", false},
		{"${XMODIFIERS%i*}", "@im=", false},
		{"${XMODIFIERS%%i*}", "@", false},
		{"${XMODIFIERS%*i}", "@im=ibus", false},
		{"${XMODIFIERS%%*i}", "@im=ibus", false},
		{"${XMODIFIERS/i/I}", "@Im=ibus", false},
		{"${XMODIFIERS//i/I}", "@Im=Ibus", false},
		{"${XMODIFIERS/#i/I}", "@im=ibus", false},
		{"${XMODIFIERS/%i/I}", "@im=ibus", false},
		{"${XMODIFIERS/#@/AT}", "ATim=ibus", false},
		{"${XMODIFIERS/#@}", "im=ibus", false},
		{"${XMODIFIERS/%s/S}", "@im=ibuS", false},
		{"${XMODIFIERS/%s}", "@im=ibu", false},
		{"${XMODIFIERS//i/aye}", "@ayem=ayebus", false},
		{"${XMODIFIERS//b/BEE}", "@im=iBEEus", false},
		{"${EDITOR/${EDITOR}/}", "", false},
		{"${EDITOR/${EDITOR}}", "", false},
		{"${EDITOR//${EDITOR}/}", "", false},
		{"${EDITOR//${EDITOR}}", "", false},
		{"${FOUREYES/ii/${EDITOR}}", "vimii", false},
		{"${FOUREYES//i/${EDITOR}}", "vimvimvimvim", false},
		{"${FOUREYES//ii/${EDITOR}}", "vimvim", false},
		{"${FOUREYES//iii/${EDITOR}}", "vimi", false},
		{"${FOUREYES//iii/${EDITOR}", "vimi", true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.pattern, func(t *testing.T) {
			actual, err := ProcessWord(testCase.pattern, envs)
			if testCase.expectError {
				require.Error(t, err)
				t.Logf("got expected error %v", err)
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.expected, actual)
			}
			// We're probably not as flexible as the shell, but at least don't be incompatible with it
			cmd := exec.Command("bash", "-c", "echo -n "+testCase.pattern)
			cmd.Env = append(cmd.Env, envs...)
			output, err := cmd.CombinedOutput()
			if testCase.expectError {
				require.Error(t, err)
				t.Logf("got expected error %v (%s)", err, string(output))
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.expected, string(output))
			}
		})
	}
}
