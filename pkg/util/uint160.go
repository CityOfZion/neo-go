package util

import (
	"encoding/hex"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/crypto"
)

const uint160Size = 20

// Uint160 is a 20 byte long unsigned integer.
type Uint160 [uint160Size]uint8

// Uint160DecodeString attempts to decode the given string into an Uint160.
func Uint160DecodeString(s string) (u Uint160, err error) {
	if len(s) != uint160Size*2 {
		return u, fmt.Errorf("expected string size of %d got %d", uint160Size*2, len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return u, err
	}
	return Uint160DecodeBytes(b)
}

// Uint160DecodeAddress attempts to decode the given NEO address string
// into an Uint160.
func Uint160DecodeAddress(s string) (u Uint160, err error) {
	b, err := crypto.Base58CheckDecode(s)
	if err != nil {
		return u, err
	}
	return Uint160DecodeBytes(b[1:21])
}

// Uint160DecodeBytes attempts to decode the given bytes into an Uint160.
func Uint160DecodeBytes(b []byte) (u Uint160, err error) {
	if len(b) != uint160Size {
		return u, fmt.Errorf("expected byte size of %d got %d", uint160Size, len(b))
	}
	for i := 0; i < uint160Size; i++ {
		u[i] = b[i]
	}
	return
}

// Bytes returns the byte slice representation of u.
func (u Uint160) Bytes() []byte {
	b := make([]byte, uint160Size)
	for i := 0; i < uint160Size; i++ {
		b[i] = byte(u[i])
	}
	return b
}

// Address returns the NEO address representation of u.
func (u Uint160) Address() string {
	// Dont forget to prepend the Address version 0x17 (23) A
	b := append([]byte{0x17}, u.Bytes()...)
	return crypto.Base58CheckEncode(b)
}

// String implements the stringer interface.
func (u Uint160) String() string {
	return hex.EncodeToString(u.Bytes())
}

// Equals returns true if both Uint256 values are the same.
func (u Uint160) Equals(other Uint160) bool {
	for i := 0; i < uint160Size; i++ {
		if u[i] != other[i] {
			return false
		}
	}
	return true
}
