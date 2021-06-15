package mptpool

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestAddRemoveUpdate(t *testing.T) {
	mp := New()

	i1 := []byte{1, 2, 3}
	i1h := util.Uint256{1, 2, 3}
	i2 := []byte{2, 3, 4}
	i2h := util.Uint256{2, 3, 4}
	i3 := []byte{3, 4, 5}
	i3h := util.Uint256{3, 4, 5}
	mapAll := map[util.Uint256][]byte{i1h: i1, i2h: i2, i3h: i3}

	// No items
	_, ok := mp.TryGet(i1h)
	require.False(t, ok)
	require.False(t, mp.ContainsKey(i1h))
	require.Equal(t, 0, mp.Count())
	require.Equal(t, map[util.Uint256][]byte{}, mp.GetAll())

	// Add i1, i2, check OK
	mp.Add(i1h, i1)
	mp.Add(i2h, i2)
	itm, ok := mp.TryGet(i1h)
	require.True(t, ok)
	require.Equal(t, i1, itm)
	require.True(t, mp.ContainsKey(i1h))
	require.True(t, mp.ContainsKey(i2h))
	require.Equal(t, map[util.Uint256][]byte{i1h: i1, i2h: i2}, mp.GetAll())
	require.Equal(t, 2, mp.Count())

	// Remove i1 and unexisting item
	mp.Remove(i3h)
	mp.Remove(i1h)
	require.False(t, mp.ContainsKey(i1h))
	require.True(t, mp.ContainsKey(i2h))
	require.Equal(t, map[util.Uint256][]byte{i2h: i2}, mp.GetAll())
	require.Equal(t, 1, mp.Count())

	// Update
	mp.Update(nil, mapAll)
	require.Equal(t, mapAll, mp.GetAll())
	require.Equal(t, 3, mp.Count())
	mp.Update(map[util.Uint256]bool{i1h: true, i2h: true, i3h: true}, mapAll)
	require.Equal(t, mapAll, mp.GetAll()) // deletion first, addition after that
	require.Equal(t, 3, mp.Count())
	mp.Update(map[util.Uint256]bool{i1h: true, i2h: true, i3h: true}, nil)
	require.Equal(t, map[util.Uint256][]byte{}, mp.GetAll())
	require.Equal(t, 0, mp.Count())
	mp.Update(map[util.Uint256]bool{i1h: true, i2h: true}, map[util.Uint256][]byte{i2h: i2, i3h: i3})
	require.Equal(t, map[util.Uint256][]byte{i2h: i2, i3h: i3}, mp.GetAll())
	require.Equal(t, 2, mp.Count())
}
