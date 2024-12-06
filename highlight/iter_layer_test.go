package highlight

import (
	"testing"
)

func Test_SortKeyCompare(t *testing.T) {
	tests := []struct {
		name     string
		k1       sortKey
		k2       sortKey
		expected int
	}{
		// Test case 1: offset differs
		{"Offset less", sortKey{offset: 1, start: false, depth: 10}, sortKey{offset: 2, start: false, depth: 10}, -1},
		{"Offset greater", sortKey{offset: 2, start: false, depth: 10}, sortKey{offset: 1, start: false, depth: 10}, 1},

		// Test case 2: offset equal, start differs
		{"Start less", sortKey{offset: 3, start: false, depth: 10}, sortKey{offset: 3, start: true, depth: 10}, -1},
		{"Start greater", sortKey{offset: 3, start: true, depth: 10}, sortKey{offset: 3, start: false, depth: 10}, 1},

		// Test case 3: offset and start equal, depth differs
		{"Depth less", sortKey{offset: 3, start: true, depth: -5}, sortKey{offset: 3, start: true, depth: 10}, -1},
		{"Depth greater", sortKey{offset: 3, start: true, depth: 10}, sortKey{offset: 3, start: true, depth: -5}, 1},

		// Test case 4: all fields equal
		{"All fields equal", sortKey{offset: 3, start: true, depth: 10}, sortKey{offset: 3, start: true, depth: 10}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.k1.Compare(tt.k2)
			if result != tt.expected {
				t.Errorf("Compare(%+v, %+v) = %d, expected %d", tt.k1, tt.k2, result, tt.expected)
			}
		})
	}
}
