package mpt

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestToNibblesFromNibbles(t *testing.T) {
	check := func(t *testing.T, expected []byte) {
		actual := fromNibbles(toNibbles(expected))
		require.Equal(t, expected, actual)
	}
	t.Run("empty path", func(t *testing.T) {
		check(t, []byte{})
	})
	t.Run("non-empty path", func(t *testing.T) {
		check(t, []byte{0x01, 0xAC, 0x8d, 0x04, 0xFF})
	})
}

func TestGetChildrenPaths(t *testing.T) {
	h1 := NewHashNode(util.Uint256{1, 2, 3})
	h2 := NewHashNode(util.Uint256{4, 5, 6})
	l := NewLeafNode([]byte{1, 2, 3})
	ext1 := NewExtensionNode([]byte{8, 9}, h1)
	ext2 := NewExtensionNode([]byte{7, 6}, l)
	branch := NewBranchNode()
	branch.Children[3] = h1
	branch.Children[5] = l
	branch.Children[lastChild] = h2
	testCases := map[string]struct {
		node     Node
		expected map[util.Uint256][]byte
	}{
		"Hash":                         {h1, nil},
		"Leaf":                         {l, nil},
		"Extension with next Hash":     {ext1, map[util.Uint256][]byte{h1.Hash(): ext1.key}},
		"Extension with next non-Hash": {ext2, map[util.Uint256][]byte{}},
		"Branch": {branch, map[util.Uint256][]byte{
			h1.Hash(): {0x03},
			h2.Hash(): {},
		}},
	}
	parentPath := []byte{4, 5, 6}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, testCase.expected, GetChildrenPaths([]byte{}, testCase.node))
			if testCase.expected != nil {
				expectedWithPrefix := make(map[util.Uint256][]byte, len(testCase.expected))
				for h, path := range testCase.expected {
					expectedWithPrefix[h] = append(parentPath, path...)
				}
				require.Equal(t, expectedWithPrefix, GetChildrenPaths(parentPath, testCase.node))
			}
		})
	}
}
