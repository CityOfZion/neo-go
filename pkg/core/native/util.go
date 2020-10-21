package native

import (
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

func getSerializableFromDAO(id int32, d dao.DAO, key []byte, item io.Serializable) error {
	si := d.GetStorageItem(id, key)
	if si == nil {
		return storage.ErrKeyNotFound
	}
	r := io.NewBinReaderFromBuf(si.Value)
	item.DecodeBinary(r)
	return r.Err
}

func nameMethod(name string) interop.Method {
	return func(_ *interop.Context, _ []stackitem.Item) stackitem.Item {
		return stackitem.NewByteArray([]byte(name))
	}
}
