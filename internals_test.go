package imagebuilder

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestMergeEnv(t *testing.T) {
	tests := [][3][]string{
		{
			[]string{"A=B", "B=C", "C=D"},
			nil,
			[]string{"A=B", "B=C", "C=D"},
		},
		{
			nil,
			[]string{"A=B", "B=C", "C=D"},
			[]string{"A=B", "B=C", "C=D"},
		},
		{
			[]string{"A=B", "B=C", "C=D", "E=F"},
			[]string{"B=O", "F=G"},
			[]string{"A=B", "B=O", "C=D", "E=F", "F=G"},
		},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			result := mergeEnv(test[0], test[1])
			if len(result) != len(test[2]) {
				t.Fatalf("expected %v, got %v", test[2], result)
			}
			for i := range result {
				if result[i] != test[2][i] {
					t.Fatalf("expected %v, got %v", test[2], result)
				}
			}
		})
	}
}

func TestEnvMapAsSlice(t *testing.T) {
	tests := [][2][]string{
		{
			[]string{"A=B", "B=C", "C=D"},
			[]string{"A=B", "B=C", "C=D"},
		},
		{
			[]string{"A=B", "B=C", "C=D", "E=F", "B=O", "F=G"},
			[]string{"A=B", "B=O", "C=D", "E=F", "F=G"},
		},
		{
			[]string{"A=B", "C=D", "B=C", "E=F", "F=G", "B=O"},
			[]string{"A=B", "B=O", "C=D", "E=F", "F=G"},
		},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			m := make(map[string]string)
			for _, spec := range test[0] {
				s := strings.SplitN(spec, "=", 2)
				m[s[0]] = s[1]
			}
			result := envMapAsSlice(m)
			sort.Strings(result)
			if len(result) != len(test[1]) {
				t.Fatalf("expected %v, got %v", test[1], result)
			}
			for i := range result {
				if result[i] != test[1][i] {
					t.Fatalf("expected %v, got %v", test[1], result)
				}
			}
		})
	}
}
