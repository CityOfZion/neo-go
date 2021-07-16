package mptpool

import (
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
)

// Pool stores unknown MPT nodes along with the corresponding paths.
type Pool struct {
	hashes *storage.MemoryStore

	lock            sync.RWMutex
	resendThreshold time.Duration
	resendFunc      func(map[util.Uint256][]byte)

	batchCh          chan struct{}
	retransmissionOn *atomic.Bool
}

// New returns new MPT node hashes pool using provided chain.
func New() *Pool {
	return &Pool{
		hashes:           storage.NewMemoryStore(),
		batchCh:          make(chan struct{}),
		retransmissionOn: atomic.NewBool(false),
	}
}

// ContainsKey checks if an MPT node hash is in the Pool.
func (mp *Pool) ContainsKey(hash util.Uint256) bool {
	_, err := mp.hashes.Get(hash.BytesBE())
	return err == nil
}

// TryGet returns MPT path for the specified HashNode.
func (mp *Pool) TryGet(hash util.Uint256) ([]byte, bool) {
	itm, err := mp.hashes.Get(hash.BytesBE())
	return itm, err == nil
}

// GetAll returns all paths from the pool.
func (mp *Pool) GetAll() map[util.Uint256][]byte {
	res := make(map[util.Uint256][]byte)
	mp.hashes.Seek(
		[]byte{}, func(k, v []byte) {
			hash, _ := util.Uint256DecodeBytesBE(k)
			res[hash] = v
		})
	return res
}

// Remove removes item from the pool by the specified hash.
func (mp *Pool) Remove(hash util.Uint256) {
	_ = mp.hashes.Delete(hash.BytesBE())
}

// Add adds item to the pool.
func (mp *Pool) Add(hash util.Uint256, item []byte) {
	_ = mp.hashes.Put(hash.BytesBE(), item)
}

// Update is an atomic operation and removes/adds specified items from/to the pool.
func (mp *Pool) Update(remove map[util.Uint256]bool, add map[util.Uint256][]byte) {
	batch := mp.hashes.Batch()
	for h := range remove {
		batch.Delete(h.BytesBE())
	}
	for h, itm := range add {
		batch.Put(h.BytesBE(), itm)
	}
	_ = mp.hashes.PutBatch(batch)
	if mp.retransmissionOn.Load() {
		mp.batchCh <- struct{}{}
	}
}

// Count returns the number of items in the pool.
func (mp *Pool) Count() int {
	return mp.hashes.Count()
}

// SetResendThreshold sets threshold after which MPT data requests will be considered
// stale and retransmitted by `ResendStaleItems` routine.
func (mp *Pool) SetResendThreshold(t time.Duration, f func(map[util.Uint256][]byte)) {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	mp.resendThreshold = t
	mp.resendFunc = f
}

// ResendStaleItems starts cycle that manages stale MPT data requests (must be run
// in a separate go-routine). The cycle will be automatically exited after MPT sync
// process is ended, i.e. when no nodes are left in pool after Update.
func (mp *Pool) ResendStaleItems() {
	if !mp.retransmissionOn.CAS(false, true) {
		return
	}
	timer := time.NewTimer(mp.resendThreshold)
	for {
		select {
		case <-timer.C:
			mp.resendFunc(mp.GetAll())
			timer.Reset(mp.resendThreshold)
		case <-mp.batchCh:
			if mp.Count() == 0 {
				mp.retransmissionOn.Store(false)
				return
			}
			// new batch is firstly requested by server, no need to duplicate requests
			timer.Reset(mp.resendThreshold)
		}
	}
}
