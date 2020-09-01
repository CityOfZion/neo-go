package compiler_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoragePutGet(t *testing.T) {
	src := `
		package foo

		import "github.com/nspcc-dev/neo-go/pkg/interop/storage"

		func Main() string {
			ctx := storage.GetContext()
			key := []byte("token")
			storage.Put(ctx, key, []byte("foo"))
			x := storage.Get(ctx, key)
			return x.(string)
		}
	`
	eval(t, src, []byte("foo"))
}

func TestNotify(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	func Main(arg int) {
		runtime.Notify("Event1", arg, "sum", arg+1)
		runtime.Notify("single")
	}`

	v, s := vmAndCompileInterop(t, src)
	v.Estack().PushVal(11)

	require.NoError(t, v.Run())
	require.Equal(t, 2, len(s.events))

	exp0 := []stackitem.Item{stackitem.NewBigInteger(big.NewInt(11)), stackitem.NewByteArray([]byte("sum")), stackitem.NewBigInteger(big.NewInt(12))}
	assert.Equal(t, "Event1", s.events[0].Name)
	assert.Equal(t, exp0, s.events[0].Item.Value())
	assert.Equal(t, "single", s.events[1].Name)
	assert.Equal(t, []stackitem.Item{}, s.events[1].Item.Value())
}
