package nef

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeBinary(t *testing.T) {
	script := []byte{12, 32, 84, 35, 14}
	expected := &File{
		Header: Header{
			Magic:    Magic,
			Compiler: "the best compiler ever",
			Version:  "1.2.3.4",
		},
		Script: script,
	}

	t.Run("invalid Magic", func(t *testing.T) {
		expected.Header.Magic = 123
		checkDecodeError(t, expected)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		expected.Header.Magic = Magic
		expected.Checksum = 123
		checkDecodeError(t, expected)
	})

	t.Run("invalid reserved", func(t *testing.T) {
		bytes, err := testserdes.EncodeBinary(expected)
		require.NoError(t, err)
		bytes[4+32+32] = 1 // set the first reserved byte to 1
		require.Error(t, testserdes.DecodeBinary(bytes, &File{}))
	})

	t.Run("zero-length script", func(t *testing.T) {
		expected.Script = make([]byte, 0)
		expected.Checksum = expected.CalculateChecksum()
		checkDecodeError(t, expected)
	})

	t.Run("invalid script length", func(t *testing.T) {
		newScript := make([]byte, MaxScriptLength+1)
		expected.Script = newScript
		expected.Checksum = expected.CalculateChecksum()
		checkDecodeError(t, expected)
	})

	t.Run("positive", func(t *testing.T) {
		expected.Script = script
		expected.Checksum = expected.CalculateChecksum()
		expected.Header.Magic = Magic
		testserdes.EncodeDecodeBinary(t, expected, &File{})
	})
}

func checkDecodeError(t *testing.T, expected *File) {
	bytes, err := testserdes.EncodeBinary(expected)
	require.NoError(t, err)
	require.Error(t, testserdes.DecodeBinary(bytes, &File{}))
}

func TestBytesFromBytes(t *testing.T) {
	script := []byte{12, 32, 84, 35, 14}
	expected := File{
		Header: Header{
			Magic:    Magic,
			Compiler: "the best compiler ever",
			Version:  "1.2.3.4",
		},
		Script: script,
	}
	expected.Checksum = expected.CalculateChecksum()

	bytes, err := expected.Bytes()
	require.NoError(t, err)
	actual, err := FileFromBytes(bytes)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
