package state

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NEP5Tracker contains info about a single account in a NEP5 contract.
type NEP5Tracker struct {
	// Balance is the current balance of the account.
	Balance big.Int
	// LastUpdatedBlock is a number of block when last `transfer` to or from the
	// account occurred.
	LastUpdatedBlock uint32
}

// NEP5TransferLog is a log of NEP5 token transfers for the specific command.
type NEP5TransferLog struct {
	Raw []byte
	// size is the number of NEP5Transfers written into Raw
	size int
}

// NEP5Transfer represents a single NEP5 Transfer event.
type NEP5Transfer struct {
	// Asset is a NEP5 contract ID.
	Asset int32
	// Address is the address of the sender.
	From util.Uint160
	// To is the address of the receiver.
	To util.Uint160
	// Amount is the amount of tokens transferred.
	// It is negative when tokens are sent and positive if they are received.
	Amount big.Int
	// Block is a number of block when the event occurred.
	Block uint32
	// Timestamp is the timestamp of the block where transfer occurred.
	Timestamp uint64
	// Tx is a hash the transaction.
	Tx util.Uint256
}

// NEP5Balances is a map of the NEP5 contract IDs
// to the corresponding structures.
type NEP5Balances struct {
	Trackers map[int32]NEP5Tracker
	// NextTransferBatch stores an index of the next transfer batch.
	NextTransferBatch uint32
}

// NewNEP5Balances returns new NEP5Balances.
func NewNEP5Balances() *NEP5Balances {
	return &NEP5Balances{
		Trackers: make(map[int32]NEP5Tracker),
	}
}

// DecodeBinary implements io.Serializable interface.
func (bs *NEP5Balances) DecodeBinary(r *io.BinReader) {
	bs.NextTransferBatch = r.ReadU32LE()
	lenBalances := r.ReadVarUint()
	m := make(map[int32]NEP5Tracker, lenBalances)
	for i := 0; i < int(lenBalances); i++ {
		key := int32(r.ReadU32LE())
		var tr NEP5Tracker
		tr.DecodeBinary(r)
		m[key] = tr
	}
	bs.Trackers = m
}

// EncodeBinary implements io.Serializable interface.
func (bs *NEP5Balances) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(bs.NextTransferBatch)
	w.WriteVarUint(uint64(len(bs.Trackers)))
	for k, v := range bs.Trackers {
		w.WriteU32LE(uint32(k))
		v.EncodeBinary(w)
	}
}

// Append appends single transfer to a log.
func (lg *NEP5TransferLog) Append(tr *NEP5Transfer) error {
	w := io.NewBufBinWriter()
	tr.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	lg.Raw = append(lg.Raw, w.Bytes()...)
	lg.size++
	return nil
}

// ForEach iterates over transfer log returning on first error.
func (lg *NEP5TransferLog) ForEach(f func(*NEP5Transfer) error) error {
	if lg == nil {
		return nil
	}
	tr := new(NEP5Transfer)
	var bytesRead int
	for i := 0; i < len(lg.Raw); i += bytesRead {
		r := io.NewBinReaderFromBuf(lg.Raw[i:])
		bytesRead = tr.DecodeBinaryReturnCount(r)
		if r.Err != nil {
			return r.Err
		} else if err := f(tr); err != nil {
			return nil
		}
	}
	return nil
}

// Size returns an amount of transfer written in log.
func (lg *NEP5TransferLog) Size() int {
	return lg.size
}

// EncodeBinary implements io.Serializable interface.
func (t *NEP5Tracker) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(bigint.ToBytes(&t.Balance))
	w.WriteU32LE(t.LastUpdatedBlock)
}

// DecodeBinary implements io.Serializable interface.
func (t *NEP5Tracker) DecodeBinary(r *io.BinReader) {
	t.Balance = *bigint.FromBytes(r.ReadVarBytes())
	t.LastUpdatedBlock = r.ReadU32LE()
}

// EncodeBinary implements io.Serializable interface.
func (t *NEP5Transfer) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(uint32(t.Asset))
	w.WriteBytes(t.Tx[:])
	w.WriteBytes(t.From[:])
	w.WriteBytes(t.To[:])
	w.WriteU32LE(t.Block)
	w.WriteU64LE(t.Timestamp)
	amountBytes := bigint.ToBytes(&t.Amount)
	w.WriteU64LE(uint64(len(amountBytes)))
	w.WriteBytes(amountBytes)
}

// DecodeBinary implements io.Serializable interface.
func (t *NEP5Transfer) DecodeBinary(r *io.BinReader) {
	_ = t.DecodeBinaryReturnCount(r)
}

// DecodeBinaryReturnCount decodes NEP5Transfer and returns the number of bytes read.
func (t *NEP5Transfer) DecodeBinaryReturnCount(r *io.BinReader) int {
	t.Asset = int32(r.ReadU32LE())
	r.ReadBytes(t.Tx[:])
	r.ReadBytes(t.From[:])
	r.ReadBytes(t.To[:])
	t.Block = r.ReadU32LE()
	t.Timestamp = r.ReadU64LE()
	amountLen := r.ReadU64LE()
	amountBytes := make([]byte, amountLen)
	r.ReadBytes(amountBytes)
	t.Amount = *bigint.FromBytes(amountBytes)
	return 4 + util.Uint160Size*2 + 8 + 4 + (8 + len(amountBytes)) + +util.Uint256Size
}
