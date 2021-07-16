package mpt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Trie is an MPT trie storing all key-value pairs.
type Trie struct {
	Store *storage.MemCachedStore

	root            Node
	refcountEnabled bool
	refcount        map[util.Uint256]*cachedNode
}

type cachedNode struct {
	bytes    []byte
	initial  int32
	refcount int32
}

var (
	// ErrNotFound is returned when requested trie item is missing.
	ErrNotFound = errors.New("item not found")
	// ErrRestoreFailed is returned when replacing HashNode by its "unhashed"
	// candidate fails.
	ErrRestoreFailed = errors.New("failed to restore HashNode")
)

// NewTrie returns new MPT trie. It accepts a MemCachedStore to decouple storage errors from logic errors
// so that all storage errors are processed during `store.Persist()` at the caller.
// This also has the benefit, that every `Put` can be considered an atomic operation.
func NewTrie(root Node, enableRefCount bool, store *storage.MemCachedStore) *Trie {
	if root == nil {
		root = new(HashNode)
	}

	return &Trie{
		Store: store,
		root:  root,

		refcountEnabled: enableRefCount,
		refcount:        make(map[util.Uint256]*cachedNode),
	}
}

// Get returns value for the provided key in t.
func (t *Trie) Get(key []byte) ([]byte, error) {
	path := toNibbles(key)
	r, bs, err := t.getWithPath(t.root, path)
	if err != nil {
		return nil, err
	}
	t.root = r
	return bs, nil
}

// getWithPath returns value the provided path in a subtrie rooting in curr.
// It also returns a current node with all hash nodes along the path
// replaced to their "unhashed" counterparts.
func (t *Trie) getWithPath(curr Node, path []byte) (Node, []byte, error) {
	switch n := curr.(type) {
	case *LeafNode:
		if len(path) == 0 {
			return curr, copySlice(n.value), nil
		}
	case *BranchNode:
		i, path := splitPath(path)
		r, bs, err := t.getWithPath(n.Children[i], path)
		if err != nil {
			return nil, nil, err
		}
		n.Children[i] = r
		return n, bs, nil
	case *HashNode:
		if !n.IsEmpty() {
			if r, err := t.getFromStore(n.hash); err == nil {
				return t.getWithPath(r, path)
			}
		}
	case *ExtensionNode:
		if bytes.HasPrefix(path, n.key) {
			r, bs, err := t.getWithPath(n.next, path[len(n.key):])
			if err != nil {
				return nil, nil, err
			}
			n.next = r
			return curr, bs, err
		}
	default:
		panic("invalid MPT node type")
	}
	return curr, nil, ErrNotFound
}

// Put puts key-value pair in t.
func (t *Trie) Put(key, value []byte) error {
	if len(key) == 0 {
		return errors.New("key is empty")
	} else if len(key) > MaxKeyLength {
		return errors.New("key is too big")
	} else if len(value) > MaxValueLength {
		return errors.New("value is too big")
	}
	if len(value) == 0 {
		return t.Delete(key)
	}
	path := toNibbles(key)
	n := NewLeafNode(value)
	r, err := t.putIntoNode(t.root, path, n, false)
	if err != nil {
		return err
	}
	t.root = r
	return nil
}

// RestoreHashNode replaces HashNode located at the provided path by the specified Node
// and stores it.
func (t *Trie) RestoreHashNode(path []byte, node Node) error {
	if _, ok := node.(*HashNode); ok {
		return fmt.Errorf("%w: unable to restore node into HashNode or EmptyNode", ErrRestoreFailed)
	}
	r, err := t.putIntoNode(t.root, path, node, true)
	if err != nil {
		return err
	}
	t.root = r

	// Put it in the storage right now, because sync process can be interrupted before
	// mpt.Flush() invocation.
	key := makeStorageKey(node.Hash().BytesBE())
	if _, err := t.Store.Get(key); err != nil {
		value := node.Bytes()
		if t.refcountEnabled {
			value = append(value, 0, 0, 0, 0)
		}
		_ = t.Store.Put(key, value)
	}

	if leaf, ok := node.(*LeafNode); ok {
		k := append([]byte{byte(storage.STStorage)}, fromNibbles(path)...)
		_ = t.Store.Put(k, leaf.value)
	}
	return nil
}

// putIntoLeaf puts val to trie if current node is a Leaf.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoLeaf(curr *LeafNode, path []byte, val Node, restore bool) (Node, error) {
	if restore {
		if len(path) == 0 {
			if curr.Hash() == val.Hash() {
				// this node has already been restored
				return curr, nil
			}
			return nil, fmt.Errorf("%w: bad Leaf node hash: expected %s, got %s", ErrRestoreFailed, curr.Hash().StringBE(), val.Hash().StringBE())
		}
		return nil, fmt.Errorf("%w: can't restore LeafNode", ErrRestoreFailed)
	}
	v := val.(*LeafNode)
	if len(path) == 0 {
		t.removeRef(curr.Hash(), curr.bytes)
		t.addRef(val.Hash(), val.Bytes())
		return v, nil
	}

	b := NewBranchNode()
	b.Children[path[0]] = t.newSubTrie(path[1:], v, true)
	b.Children[lastChild] = curr
	t.addRef(b.Hash(), b.bytes)
	return b, nil
}

// putIntoBranch puts val to trie if current node is a Branch.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoBranch(curr *BranchNode, path []byte, val Node, restore bool) (Node, error) {
	if restore && len(path) == 0 && curr.Hash().Equals(val.Hash()) {
		// this node has already been restored
		return curr, nil
	}
	i, path := splitPath(path)
	t.removeRef(curr.Hash(), curr.bytes)
	r, err := t.putIntoNode(curr.Children[i], path, val, restore)
	if err != nil {
		if restore {
			t.addRef(curr.Hash(), curr.bytes)
		}
		return nil, err
	}
	curr.Children[i] = r
	curr.invalidateCache()
	t.addRef(curr.Hash(), curr.bytes)
	return curr, nil
}

// putIntoExtension puts val to trie if current node is an Extension.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoExtension(curr *ExtensionNode, path []byte, val Node, restore bool) (Node, error) {
	t.removeRef(curr.Hash(), curr.bytes)
	if bytes.HasPrefix(path, curr.key) {
		r, err := t.putIntoNode(curr.next, path[len(curr.key):], val, restore)
		if err != nil {
			if restore {
				t.addRef(curr.Hash(), curr.bytes)
			}
			return nil, err
		}
		curr.next = r
		curr.invalidateCache()
		t.addRef(curr.Hash(), curr.bytes)
		return curr, nil
	}

	if restore {
		if len(path) == 0 {
			if curr.Hash() == val.Hash() {
				// this node has already been restored
				t.addRef(curr.Hash(), curr.bytes)
				return curr, nil
			}
			return nil, fmt.Errorf("%w: bad Extension node hash: expected %s, got %s", ErrRestoreFailed, curr.Hash().StringBE(), val.Hash().StringBE())
		}
		return nil, fmt.Errorf("%w: can't modify ExtensionNode during restore", ErrRestoreFailed)
	}

	pref := lcp(curr.key, path)
	lp := len(pref)
	keyTail := curr.key[lp:]
	pathTail := path[lp:]

	s1 := t.newSubTrie(keyTail[1:], curr.next, false)
	b := NewBranchNode()
	b.Children[keyTail[0]] = s1

	i, pathTail := splitPath(pathTail)
	s2 := t.newSubTrie(pathTail, val, true)
	b.Children[i] = s2

	t.addRef(b.Hash(), b.bytes)
	if lp > 0 {
		e := NewExtensionNode(copySlice(pref), b)
		t.addRef(e.Hash(), e.bytes)
		return e, nil
	}
	return b, nil
}

// putIntoHash puts val to trie if current node is a HashNode.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoHash(curr *HashNode, path []byte, val Node, restore bool) (Node, error) {
	if curr.IsEmpty() {
		if restore {
			return nil, fmt.Errorf("%w: can't restore empty HashNode", ErrRestoreFailed)
		}
		hn := t.newSubTrie(path, val, true)
		return hn, nil
	}

	if restore && len(path) == 0 {
		// `curr` hash node can be either of
		// 1) saved in storage (i.e. if this part of MPT is untouched after previous changes),
		//    because we don't remove old MPT nodes while syncing MPT state via P2P, so just
		//    add it to local in-memory MPT.
		// 2) missing from the storage. It's OK because we're syncing MPT state, and the purpose
		//    is to store missing hash nodes.
		// both cases are OK, but we still need to validate `val` against `curr`.
		if val.Hash() != curr.Hash() {
			return nil, fmt.Errorf("%w: can't restore HashNode: expected and actual hashes mismatch (%s vs %s)", ErrRestoreFailed, curr.Hash().StringBE(), val.Hash().StringBE())
		}
		// We also need to add corresponding refcount in both cases, so treat `val` as new value.
		hn := t.newSubTrie(path, val, true)
		return hn, nil
	}
	result, err := t.getFromStore(curr.hash)
	if err != nil {
		return nil, err
	}
	return t.putIntoNode(result, path, val, restore)
}

// newSubTrie create new trie containing node at provided path.
func (t *Trie) newSubTrie(path []byte, val Node, newVal bool) Node {
	if newVal {
		t.addRef(val.Hash(), val.Bytes())
	}
	if len(path) == 0 {
		return val
	}
	e := NewExtensionNode(path, val)
	t.addRef(e.Hash(), e.bytes)
	return e
}

// putIntoNode puts val with provided path inside curr and returns updated node.
// Reference counters are updated for both curr and returned value.
func (t *Trie) putIntoNode(curr Node, path []byte, val Node, restore bool) (Node, error) {
	switch n := curr.(type) {
	case *LeafNode:
		return t.putIntoLeaf(n, path, val, restore)
	case *BranchNode:
		return t.putIntoBranch(n, path, val, restore)
	case *ExtensionNode:
		return t.putIntoExtension(n, path, val, restore)
	case *HashNode:
		return t.putIntoHash(n, path, val, restore)
	default:
		panic("invalid MPT node type")
	}
}

// Delete removes key from trie.
// It returns no error on missing key.
func (t *Trie) Delete(key []byte) error {
	path := toNibbles(key)
	r, err := t.deleteFromNode(t.root, path)
	if err != nil {
		return err
	}
	t.root = r
	return nil
}

func (t *Trie) deleteFromBranch(b *BranchNode, path []byte) (Node, error) {
	i, path := splitPath(path)
	h := b.Hash()
	bs := b.bytes
	r, err := t.deleteFromNode(b.Children[i], path)
	if err != nil {
		return nil, err
	}
	t.removeRef(h, bs)
	b.Children[i] = r
	b.invalidateCache()
	var count, index int
	for i := range b.Children {
		h, ok := b.Children[i].(*HashNode)
		if !ok || !h.IsEmpty() {
			index = i
			count++
		}
	}
	// count is >= 1 because branch node had at least 2 children before deletion.
	if count > 1 {
		t.addRef(b.Hash(), b.bytes)
		return b, nil
	}
	c := b.Children[index]
	if index == lastChild {
		return c, nil
	}
	if h, ok := c.(*HashNode); ok {
		c, err = t.getFromStore(h.Hash())
		if err != nil {
			return nil, err
		}
	}
	if e, ok := c.(*ExtensionNode); ok {
		t.removeRef(e.Hash(), e.bytes)
		e.key = append([]byte{byte(index)}, e.key...)
		e.invalidateCache()
		t.addRef(e.Hash(), e.bytes)
		return e, nil
	}

	e := NewExtensionNode([]byte{byte(index)}, c)
	t.addRef(e.Hash(), e.bytes)
	return e, nil
}

func (t *Trie) deleteFromExtension(n *ExtensionNode, path []byte) (Node, error) {
	if !bytes.HasPrefix(path, n.key) {
		return n, nil
	}
	h := n.Hash()
	bs := n.bytes
	r, err := t.deleteFromNode(n.next, path[len(n.key):])
	if err != nil {
		return nil, err
	}
	t.removeRef(h, bs)
	switch nxt := r.(type) {
	case *ExtensionNode:
		t.removeRef(nxt.Hash(), nxt.bytes)
		n.key = append(n.key, nxt.key...)
		n.next = nxt.next
	case *HashNode:
		if nxt.IsEmpty() {
			return nxt, nil
		}
		n.next = nxt
	default:
		n.next = r
	}
	n.invalidateCache()
	t.addRef(n.Hash(), n.bytes)
	return n, nil
}

// deleteFromNode removes value with provided path from curr and returns an updated node.
// Reference counters are updated for both curr and returned value.
func (t *Trie) deleteFromNode(curr Node, path []byte) (Node, error) {
	switch n := curr.(type) {
	case *LeafNode:
		if len(path) == 0 {
			t.removeRef(curr.Hash(), curr.Bytes())
			return new(HashNode), nil
		}
		return curr, nil
	case *BranchNode:
		return t.deleteFromBranch(n, path)
	case *ExtensionNode:
		return t.deleteFromExtension(n, path)
	case *HashNode:
		if n.IsEmpty() {
			return n, nil
		}
		newNode, err := t.getFromStore(n.Hash())
		if err != nil {
			return nil, err
		}
		return t.deleteFromNode(newNode, path)
	default:
		panic("invalid MPT node type")
	}
}

// StateRoot returns root hash of t.
func (t *Trie) StateRoot() util.Uint256 {
	if hn, ok := t.root.(*HashNode); ok && hn.IsEmpty() {
		return util.Uint256{}
	}
	return t.root.Hash()
}

func makeStorageKey(mptKey []byte) []byte {
	return append([]byte{byte(storage.DataMPT)}, mptKey...)
}

// Flush puts every node in the trie except Hash ones to the storage.
// Because we care only about block-level changes, there is no need to put every
// new node to storage. Normally, flush should be called with every StateRoot persist, i.e.
// after every block.
func (t *Trie) Flush() {
	for h, node := range t.refcount {
		if node.refcount != 0 {
			if node.bytes == nil {
				panic("item not in trie")
			}
			if t.refcountEnabled {
				node.initial = t.updateRefCount(h)
				if node.initial == 0 {
					delete(t.refcount, h)
				}
			} else if node.refcount > 0 {
				_ = t.Store.Put(makeStorageKey(h.BytesBE()), node.bytes)
			}
			node.refcount = 0
		} else {
			delete(t.refcount, h)
		}
	}
}

// updateRefCount should be called only when refcounting is enabled.
func (t *Trie) updateRefCount(h util.Uint256) int32 {
	if !t.refcountEnabled {
		panic("`updateRefCount` is called, but GC is disabled")
	}
	var data []byte
	key := makeStorageKey(h.BytesBE())
	node := t.refcount[h]
	cnt := node.initial
	if cnt == 0 {
		// A newly created item which may be in store.
		var err error
		data, err = t.Store.Get(key)
		if err == nil {
			cnt = int32(binary.LittleEndian.Uint32(data[len(data)-4:]))
		}
	}
	if len(data) == 0 {
		data = append(node.bytes, 0, 0, 0, 0)
	}
	cnt += node.refcount
	switch {
	case cnt < 0:
		// BUG: negative reference count
		panic(fmt.Sprintf("negative reference count: %s new %d, upd %d", h.StringBE(), cnt, t.refcount[h]))
	case cnt == 0:
		_ = t.Store.Delete(key)
	default:
		binary.LittleEndian.PutUint32(data[len(data)-4:], uint32(cnt))
		_ = t.Store.Put(key, data)
	}
	return cnt
}

func (t *Trie) addRef(h util.Uint256, bs []byte) {
	node := t.refcount[h]
	if node == nil {
		t.refcount[h] = &cachedNode{
			refcount: 1,
			bytes:    bs,
		}
		return
	}
	node.refcount++
	if node.bytes == nil {
		node.bytes = bs
	}
}

func (t *Trie) removeRef(h util.Uint256, bs []byte) {
	node := t.refcount[h]
	if node == nil {
		t.refcount[h] = &cachedNode{
			refcount: -1,
			bytes:    bs,
		}
		return
	}
	node.refcount--
	if node.bytes == nil {
		node.bytes = bs
	}
}

func (t *Trie) getFromStore(h util.Uint256) (Node, error) {
	data, err := t.Store.Get(makeStorageKey(h.BytesBE()))
	if err != nil {
		return nil, err
	}

	var n NodeObject
	r := io.NewBinReaderFromBuf(data)
	n.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}

	if t.refcountEnabled {
		data = data[:len(data)-4]
		node := t.refcount[h]
		if node != nil {
			node.bytes = data
			node.initial = int32(r.ReadU32LE())
		}
	}
	n.Node.(flushedNode).setCache(data, h)
	return n.Node, nil
}

// Collapse compresses all nodes at depth n to the hash nodes.
// Note: this function does not perform any kind of storage flushing so
// `Flush()` should be called explicitly before invoking function.
func (t *Trie) Collapse(depth int) {
	if depth < 0 {
		panic("negative depth")
	}
	t.root = collapse(depth, t.root)
	t.refcount = make(map[util.Uint256]*cachedNode)
}

func collapse(depth int, node Node) Node {
	if _, ok := node.(*HashNode); ok {
		return node
	} else if depth == 0 {
		return NewHashNode(node.Hash())
	}

	switch n := node.(type) {
	case *BranchNode:
		for i := range n.Children {
			n.Children[i] = collapse(depth-1, n.Children[i])
		}
	case *ExtensionNode:
		n.next = collapse(depth-1, n.next)
	case *LeafNode:
	case *HashNode:
	default:
		panic("invalid MPT node type")
	}
	return node
}
