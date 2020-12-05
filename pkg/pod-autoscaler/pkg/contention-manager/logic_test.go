package contentionmanager

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProportional(t *testing.T) {

	testcases := []struct {
		description    string
		desired        int64
		desiredTotal   int64
		totalAvailable int64
		expected       int64
	}{
		{
			description:    "should get half of the resources",
			desired:        2,
			desiredTotal:   4,
			totalAvailable: 2,
			expected:       1,
		},
		{
			description:    "should get all the resources",
			desired:        2,
			desiredTotal:   2,
			totalAvailable: 1,
			expected:       1,
		},
		{
			description:    "should get no the resources",
			desired:        0,
			desiredTotal:   2,
			totalAvailable: 1,
			expected:       0,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := proportional(tt.desired, tt.desiredTotal, tt.totalAvailable)
			require.Equal(t, tt.expected, actual)
		})
	}
}
