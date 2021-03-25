package payload

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Transactions represents batch of transactions.
type Transactions struct {
	Network netmode.Magic
	Values  []*transaction.Transaction
}

// MaxBatchSize is maximum amount of transactions in batch.
const MaxBatchSize = 128

// DecodeBinary implements io.Serializable interface.
func (t *Transactions) DecodeBinary(r *io.BinReader) {
	l := r.ReadVarUint()
	if l == 0 {
		r.Err = errors.New("empty batch")
		return
	}
	if l > MaxBatchSize {
		r.Err = errors.New("batch is too big")
		return
	}
	for i := uint64(0); i < l; i++ {
		tx := &transaction.Transaction{Network: t.Network}
		tx.DecodeBinary(r)
		if r.Err != nil {
			return
		}
		t.Values = append(t.Values, tx)
	}
}

// EncodeBinary implements io.Serializable interface.
func (t *Transactions) EncodeBinary(w *io.BinWriter) {
	w.WriteArray(t.Values)
}
