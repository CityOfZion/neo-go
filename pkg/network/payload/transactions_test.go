package payload

import (
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/stretchr/testify/require"
)

func TestTransactionsSerializable(t *testing.T) {
	getTx := func() *transaction.Transaction {
		tx := transaction.New(netmode.UnitTestNet, []byte{1}, rand.Int63n(100)+1)
		tx.Signers = []transaction.Signer{{Account: random.Uint160()}}
		tx.Scripts = []transaction.Witness{{
			InvocationScript:   random.Bytes(2),
			VerificationScript: random.Bytes(3),
		}}
		tx.Hash()
		tx.Size()
		return tx
	}

	t.Run("good", func(t *testing.T) {
		txs := &Transactions{
			Network: netmode.UnitTestNet,
			Values:  []*transaction.Transaction{getTx(), getTx()},
		}
		testserdes.EncodeDecodeBinary(t, txs, &Transactions{Network: netmode.UnitTestNet})
	})
	t.Run("empty", func(t *testing.T) {
		txs := &Transactions{Network: netmode.UnitTestNet}
		data, err := testserdes.EncodeBinary(txs)
		require.NoError(t, err)
		require.Error(t, testserdes.DecodeBinary(data, &Transactions{Network: netmode.UnitTestNet}))
	})
	t.Run("too big", func(t *testing.T) {
		txs := &Transactions{Network: netmode.UnitTestNet}
		for i := 0; i <= MaxBatchSize; i++ {
			txs.Values = append(txs.Values, getTx())
		}
		data, err := testserdes.EncodeBinary(txs)
		require.NoError(t, err)
		require.Error(t, testserdes.DecodeBinary(data, &Transactions{Network: netmode.UnitTestNet}))
	})
	t.Run("invalid tx", func(t *testing.T) {
		require.Error(t, testserdes.DecodeBinary([]byte{1}, &Transactions{Network: netmode.UnitTestNet}))
	})
}
