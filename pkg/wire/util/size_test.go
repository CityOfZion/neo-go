package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVarSize(t *testing.T) {
	testCases := []struct {
		variable interface{}
		name     string
		expected int
	}{
		{
			252,
			"test_int_1",
			1,
		},
		{
			253,
			"test_int_2",
			3,
		},
		{
			65535,
			"test_int_3",
			3,
		},
		{
			65536,
			"test_int_4",
			5,
		},
		{
			4294967295,
			"test_int_5",
			5,
		},
		{
			[]byte{1, 2, 4, 5, 6},
			"test_[]byte_1",
			6,
		},
		{
			// The neo C# implementation doe not allowed this!
			Uint160{1, 2, 4, 5, 6},
			"test_Uint160_1",
			21,
		},

		{[20]uint8{1, 2, 3, 4, 5, 6},
			"test_uint8_1",
			21,
		},
		{[20]uint8{1, 2, 3, 4, 5, 6, 8, 9},
			"test_uint8_2",
			21,
		},

		{[32]uint8{1, 2, 3, 4, 5, 6},
			"test_uint8_3",
			33,
		},
		{[10]uint16{1, 2, 3, 4, 5, 6},
			"test_uint16_1",
			21,
		},

		{[10]uint16{1, 2, 3, 4, 5, 6, 10, 21},
			"test_uint16_2",
			21,
		},
		{[30]uint32{1, 2, 3, 4, 5, 6, 10, 21},
			"test_uint32_2",
			121,
		},
		{[30]uint64{1, 2, 3, 4, 5, 6, 10, 21},
			"test_uint64_2",
			241,
		},
		{[20]int8{1, 2, 3, 4, 5, 6},
			"test_int8_1",
			21,
		},
		{[20]int8{-1, 2, 3, 4, 5, 6, 8, 9},
			"test_int8_2",
			21,
		},

		{[32]int8{-1, 2, 3, 4, 5, 6},
			"test_int8_3",
			33,
		},
		{[10]int16{-1, 2, 3, 4, 5, 6},
			"test_int16_1",
			21,
		},

		{[10]int16{-1, 2, 3, 4, 5, 6, 10, 21},
			"test_int16_2",
			21,
		},
		{[30]int32{-1, 2, 3, 4, 5, 6, 10, 21},
			"test_int32_2",
			121,
		},
		{[30]int64{-1, 2, 3, 4, 5, 6, 10, 21},
			"test_int64_2",
			241,
		},
		// The neo C# implementation doe not allowed this!
		{Uint256{1, 2, 3, 4, 5, 6},
			"test_Uint256_1",
			33,
		},

		{"abc",
			"test_string_1",
			4,
		},
		{"abcà",
			"test_string_2",
			6,
		},
		{"2d3b96ae1bcc5a585e075e3b81920210dec16302",
			"test_string_3",
			41,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("run: %s", tc.name), func(t *testing.T) {
			result := GetVarSize(tc.variable)
			assert.Equal(t, tc.expected, result)
		})
	}
}
