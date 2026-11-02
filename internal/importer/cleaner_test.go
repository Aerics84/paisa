package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanPayee_RemovesBankNoiseAndIDs(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			input:    "UPI-628488132438-AXB20261011065534179345543681884180_UPI-628488132438-Smart Ba",
			expected: "Smart Ba",
		},
		{
			input:    "UPI-153753889520-UPI5ca7522e9efb5a2887d8e2ac5a2bf462_UPI-153753889520-ZEPTONOW",
			expected: "ZEPTONOW",
		},
		{
			input:    "UPI-628483076887-AXB20261011065534179345543681884170_UPI-628483076887-SWIGGY  ",
			expected: "SWIGGY",
		},
		{
			input:    "UPI-628476888036-RBA01a128edf6bbb180c3c81ac01df186f8_UPI-628476888036-Amazon I",
			expected: "Amazon I",
		},
		{
			input:    "UPI-628476639026-RBA01a1271bf68fe97d2e804990fc5898e2_UPI-628476639026-Amazon B",
			expected: "Amazon B",
		},
		{
			input:    "UPI-628476637684-APL01a1271ada1c694c5e121259e7805ae8_UPI-628476637684-Amazon I",
			expected: "Amazon I",
		},
		{
			input:    "UPI-Smart Ba-14336279452-REF998877",
			expected: "Smart Ba",
		},
		{
			input:    "UPI/SWIGGY/14333153570/swiggy@icici",
			expected: "SWIGGY",
		},
		{
			input:    "BBPS Payment received 14238609506",
			expected: "Payment received",
		},
	}

	for _, tc := range cases {
		actual := CleanPayee(tc.input)
		assert.Equal(t, tc.expected, actual, "Failed on input: %s", tc.input)
	}
}
