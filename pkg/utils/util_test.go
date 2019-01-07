package utils

import "testing"

func TestMapInMap(t *testing.T) {
	type test struct {
		name       string
		labels     map[string]string
		properties map[string]string
		expected   bool
	}

	tests := []test{
		{
			name:       "label is empty",
			labels:     map[string]string{},
			properties: map[string]string{},
			expected:   true,
		},
		{
			name: "properties contains label",
			labels: map[string]string{
				"type": "k80",
			},
			properties: map[string]string{
				"type":  "k80",
				"model": "nvidia.com/gpu",
			},
			expected: true,
		},
		{
			name: "properties cannot contains label",
			labels: map[string]string{
				"type": "k80",
				"size": "100",
			},
			properties: map[string]string{
				"type":  "k80",
				"model": "nvidia.com/gpu",
			},
			expected: false,
		},
	}
	for _, test := range tests {
		actually := MapInMap(test.labels, test.properties)
		if actually != test.expected {
			t.Errorf("expected: %v, but actually: %v", test.expected, actually)
		}
	}
}
