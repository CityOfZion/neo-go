package mempool

import (
	"errors"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

var (
	// ErrInsufficientFunds is returned when Sender is not able to pay for
	// transaction being added irrespective of the other contents of the
	// pool.
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrConflict is returned when transaction being added is incompatible
	// with the contents of the memory pool (Sender doesn't have enough GAS
	// to pay for all transactions in the pool).
	ErrConflict = errors.New("conflicts with the memory pool")
	// ErrDup is returned when transaction being added is already present
	// in the memory pool.
	ErrDup = errors.New("already in the memory pool")
	// ErrOOM is returned when transaction just doesn't fit in the memory
	// pool because of its capacity constraints.
	ErrOOM = errors.New("out of memory")
)

// item represents a transaction in the the Memory pool.
type item struct {
	txn       *transaction.Transaction
	timeStamp time.Time
}

// items is a slice of item.
type items []*item

// utilityBalanceAndFees stores sender's balance and overall fees of
// sender's transactions which are currently in mempool
type utilityBalanceAndFees struct {
	balance *big.Int
	feeSum  *big.Int
}

// Pool stores the unconfirms transactions.
type Pool struct {
	lock         sync.RWMutex
	verifiedMap  map[util.Uint256]*item
	verifiedTxes items
	fees         map[util.Uint160]utilityBalanceAndFees

	capacity   int
	feePerByte int64
}

func (p items) Len() int           { return len(p) }
func (p items) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p items) Less(i, j int) bool { return p[i].CompareTo(p[j]) < 0 }

// CompareTo returns the difference between two items.
// difference < 0 implies p < otherP.
// difference = 0 implies p = otherP.
// difference > 0 implies p > otherP.
func (p *item) CompareTo(otherP *item) int {
	if otherP == nil {
		return 1
	}

	pHigh := p.txn.HasAttribute(transaction.HighPriority)
	otherHigh := otherP.txn.HasAttribute(transaction.HighPriority)
	if pHigh && !otherHigh {
		return 1
	} else if !pHigh && otherHigh {
		return -1
	}

	// Fees sorted ascending.
	if ret := int(p.txn.FeePerByte() - otherP.txn.FeePerByte()); ret != 0 {
		return ret
	}

	if ret := int(p.txn.NetworkFee - otherP.txn.NetworkFee); ret != 0 {
		return ret
	}

	// Transaction hash sorted descending.
	return otherP.txn.Hash().CompareTo(p.txn.Hash())
}

// Count returns the total number of uncofirm transactions.
func (mp *Pool) Count() int {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	return mp.count()
}

// count is an internal unlocked version of Count.
func (mp *Pool) count() int {
	return len(mp.verifiedTxes)
}

// ContainsKey checks if a transactions hash is in the Pool.
func (mp *Pool) ContainsKey(hash util.Uint256) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	return mp.containsKey(hash)
}

// containsKey is an internal unlocked version of ContainsKey.
func (mp *Pool) containsKey(hash util.Uint256) bool {
	if _, ok := mp.verifiedMap[hash]; ok {
		return true
	}

	return false
}

// tryAddSendersFee tries to add system fee and network fee to the total sender`s fee in mempool
// and returns false if both balance check is required and sender has not enough GAS to pay
func (mp *Pool) tryAddSendersFee(tx *transaction.Transaction, feer Feer, needCheck bool) bool {
	senderFee, ok := mp.fees[tx.Sender()]
	if !ok {
		senderFee.balance = feer.GetUtilityTokenBalance(tx.Sender())
		senderFee.feeSum = big.NewInt(0)
		mp.fees[tx.Sender()] = senderFee
	}
	if needCheck && checkBalance(tx, senderFee) != nil {
		return false
	}
	senderFee.feeSum.Add(senderFee.feeSum, big.NewInt(tx.SystemFee+tx.NetworkFee))
	mp.fees[tx.Sender()] = senderFee
	return true
}

// checkBalance returns nil in case when sender has enough GAS to pay for the
// transaction
func checkBalance(tx *transaction.Transaction, balance utilityBalanceAndFees) error {
	txFee := big.NewInt(tx.SystemFee + tx.NetworkFee)
	if balance.balance.Cmp(txFee) < 0 {
		return ErrInsufficientFunds
	}
	needFee := txFee.Add(txFee, balance.feeSum)
	if balance.balance.Cmp(needFee) < 0 {
		return ErrConflict
	}
	return nil
}

// Add tries to add given transaction to the Pool.
func (mp *Pool) Add(t *transaction.Transaction, fee Feer) error {
	var pItem = &item{
		txn:       t,
		timeStamp: time.Now().UTC(),
	}
	mp.lock.Lock()
	if mp.containsKey(t.Hash()) {
		mp.lock.Unlock()
		return ErrDup
	}
	err := mp.checkTxConflicts(t, fee)
	if err != nil {
		mp.lock.Unlock()
		return err
	}

	mp.verifiedMap[t.Hash()] = pItem
	// Insert into sorted array (from max to min, that could also be done
	// using sort.Sort(sort.Reverse()), but it incurs more overhead. Notice
	// also that we're searching for position that is strictly more
	// prioritized than our new item because we do expect a lot of
	// transactions with the same priority and appending to the end of the
	// slice is always more efficient.
	n := sort.Search(len(mp.verifiedTxes), func(n int) bool {
		return pItem.CompareTo(mp.verifiedTxes[n]) > 0
	})

	// We've reached our capacity already.
	if len(mp.verifiedTxes) == mp.capacity {
		// Less prioritized than the least prioritized we already have, won't fit.
		if n == len(mp.verifiedTxes) {
			mp.lock.Unlock()
			return ErrOOM
		}
		// Ditch the last one.
		unlucky := mp.verifiedTxes[len(mp.verifiedTxes)-1]
		delete(mp.verifiedMap, unlucky.txn.Hash())
		mp.verifiedTxes[len(mp.verifiedTxes)-1] = pItem
	} else {
		mp.verifiedTxes = append(mp.verifiedTxes, pItem)
	}
	if n != len(mp.verifiedTxes)-1 {
		copy(mp.verifiedTxes[n+1:], mp.verifiedTxes[n:])
		mp.verifiedTxes[n] = pItem
	}
	// we already checked balance in checkTxConflicts, so don't need to check again
	mp.tryAddSendersFee(pItem.txn, fee, false)

	updateMempoolMetrics(len(mp.verifiedTxes))
	mp.lock.Unlock()
	return nil
}

// Remove removes an item from the mempool, if it exists there (and does
// nothing if it doesn't).
func (mp *Pool) Remove(hash util.Uint256) {
	mp.lock.Lock()
	if it, ok := mp.verifiedMap[hash]; ok {
		var num int
		delete(mp.verifiedMap, hash)
		for num = range mp.verifiedTxes {
			if hash.Equals(mp.verifiedTxes[num].txn.Hash()) {
				break
			}
		}
		if num < len(mp.verifiedTxes)-1 {
			mp.verifiedTxes = append(mp.verifiedTxes[:num], mp.verifiedTxes[num+1:]...)
		} else if num == len(mp.verifiedTxes)-1 {
			mp.verifiedTxes = mp.verifiedTxes[:num]
		}
		senderFee := mp.fees[it.txn.Sender()]
		senderFee.feeSum.Sub(senderFee.feeSum, big.NewInt(it.txn.SystemFee+it.txn.NetworkFee))
		mp.fees[it.txn.Sender()] = senderFee
	}
	updateMempoolMetrics(len(mp.verifiedTxes))
	mp.lock.Unlock()
}

// RemoveStale filters verified transactions through the given function keeping
// only the transactions for which it returns a true result. It's used to quickly
// drop part of the mempool that is now invalid after the block acceptance.
func (mp *Pool) RemoveStale(isOK func(*transaction.Transaction) bool, feer Feer) {
	mp.lock.Lock()
	policyChanged := mp.loadPolicy(feer)
	// We can reuse already allocated slice
	// because items are iterated one-by-one in increasing order.
	newVerifiedTxes := mp.verifiedTxes[:0]
	mp.fees = make(map[util.Uint160]utilityBalanceAndFees) // it'd be nice to reuse existing map, but we can't easily clear it
	for _, itm := range mp.verifiedTxes {
		if isOK(itm.txn) && mp.checkPolicy(itm.txn, policyChanged) && mp.tryAddSendersFee(itm.txn, feer, true) {
			newVerifiedTxes = append(newVerifiedTxes, itm)
		} else {
			delete(mp.verifiedMap, itm.txn.Hash())
		}
	}
	mp.verifiedTxes = newVerifiedTxes
	mp.lock.Unlock()
}

// loadPolicy updates feePerByte field and returns whether policy has been
// changed.
func (mp *Pool) loadPolicy(feer Feer) bool {
	newFeePerByte := feer.FeePerByte()
	if newFeePerByte > mp.feePerByte {
		mp.feePerByte = newFeePerByte
		return true
	}
	return false
}

// checkPolicy checks whether transaction fits policy.
func (mp *Pool) checkPolicy(tx *transaction.Transaction, policyChanged bool) bool {
	if !policyChanged || tx.FeePerByte() >= mp.feePerByte {
		return true
	}
	return false
}

// New returns a new Pool struct.
func New(capacity int) *Pool {
	return &Pool{
		verifiedMap:  make(map[util.Uint256]*item),
		verifiedTxes: make([]*item, 0, capacity),
		capacity:     capacity,
		fees:         make(map[util.Uint160]utilityBalanceAndFees),
	}
}

// TryGetValue returns a transaction and its fee if it exists in the memory pool.
func (mp *Pool) TryGetValue(hash util.Uint256) (*transaction.Transaction, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	if pItem, ok := mp.verifiedMap[hash]; ok {
		return pItem.txn, ok
	}

	return nil, false
}

// GetVerifiedTransactions returns a slice of transactions with their fees.
func (mp *Pool) GetVerifiedTransactions() []*transaction.Transaction {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	var t = make([]*transaction.Transaction, len(mp.verifiedTxes))

	for i := range mp.verifiedTxes {
		t[i] = mp.verifiedTxes[i].txn
	}

	return t
}

// checkTxConflicts is an internal unprotected version of Verify.
func (mp *Pool) checkTxConflicts(tx *transaction.Transaction, fee Feer) error {
	senderFee, ok := mp.fees[tx.Sender()]
	if !ok {
		senderFee.balance = fee.GetUtilityTokenBalance(tx.Sender())
		senderFee.feeSum = big.NewInt(0)
	}
	return checkBalance(tx, senderFee)
}

// Verify checks if a Sender of tx is able to pay for it (and all the other
// transactions in the pool). If yes, the transaction tx is a valid
// transaction and the function returns true. If no, the transaction tx is
// considered to be invalid the function returns false.
func (mp *Pool) Verify(tx *transaction.Transaction, feer Feer) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	return mp.checkTxConflicts(tx, feer) == nil
}
