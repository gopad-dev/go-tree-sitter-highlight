package highlight

import (
	"slices"
	"testing"
)

func TestRotateLeft(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		from     int
		to       int
		mid      int
		expected []string
	}{
		{
			name:     "rotate left by 2",
			args:     []string{"a", "b", "c", "d", "e", "f"},
			from:     -1,
			to:       -1,
			mid:      2,
			expected: []string{"c", "d", "e", "f", "a", "b"},
		},
		{
			name:     "rotate sub slice left by 1",
			args:     []string{"a", "b", "c", "d", "e", "f"},
			from:     1,
			to:       6,
			mid:      1,
			expected: []string{"a", "c", "d", "e", "f", "b"},
		},
	}

	for _, test := range tests {
		var actual []string
		if test.from == -1 && test.to == -1 {
			actual = rotateLeft(test.args, test.mid)
		} else {
			actual = append(test.args[:test.from], append(rotateLeft(test.args[test.from:test.to], test.mid), test.args[test.to:]...)...)
		}

		if !slices.Equal(actual, test.expected) {
			t.Errorf("%s: expected %v, got %v", test.name, test.expected, actual)
		}
	}
}
