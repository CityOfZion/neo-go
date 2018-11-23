package util

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint160UnmarshalJSON(t *testing.T) {
	str := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	expected, err := Uint160DecodeString(str)
	if err != nil {
		t.Fatal(err)
	}

	// UnmarshalJSON should decode hex-strings
	var u1, u2 Uint160

	if err = u1.UnmarshalJSON([]byte(`"` + str + `"`)); err != nil {
		t.Fatal(err)
	}
	assert.True(t, expected.Equals(u1))

	s, err := expected.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	// UnmarshalJSON should decode hex-strings prefixed by 0x
	if err = u2.UnmarshalJSON(s); err != nil {
		t.Fatal(err)
	}
	assert.True(t, expected.Equals(u1))
}

func TestUInt160DecodeString(t *testing.T) {
	hexStr := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	val, err := Uint160DecodeString(hexStr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, hexStr, val.String())
}

func TestUint160DecodeBytes(t *testing.T) {
	hexStr := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatal(err)
	}
	val, err := Uint160DecodeBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, hexStr, val.String())
}

func TestUInt160Equals(t *testing.T) {
	a := "2d3b96ae1bcc5a585e075e3b81920210dec16302"
	b := "4d3b96ae1bcc5a585e075e3b81920210dec16302"

	ua, err := Uint160DecodeString(a)
	if err != nil {
		t.Fatal(err)
	}
	ub, err := Uint160DecodeString(b)
	if err != nil {
		t.Fatal(err)
	}
	if ua.Equals(ub) {
		t.Fatalf("%s and %s cannot be equal", ua, ub)
	}
	if !ua.Equals(ua) {
		t.Fatalf("%s and %s must be equal", ua, ua)
	}
}
