package client

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// GetInvocationError returns an error in case of bad VM state or empty stack.
func GetInvocationError(result *result.Invoke) error {
	if result.State != "HALT" {
		return fmt.Errorf("invocation failed: %s", result.FaultException)
	}
	if len(result.Stack) == 0 {
		return errors.New("result stack is empty")
	}
	return nil
}

// TopBoolFromStack returns the top boolean value from stack.
func TopBoolFromStack(st []stackitem.Item) (bool, error) {
	index := len(st) - 1 // top stack element is last in the array
	result, ok := st[index].Value().(bool)
	if !ok {
		return false, fmt.Errorf("invalid stack item type: %s", st[index].Type())
	}
	return result, nil
}

// TopIntFromStack returns the top integer value from stack.
func TopIntFromStack(st []stackitem.Item) (int64, error) {
	index := len(st) - 1 // top stack element is last in the array
	bi, err := st[index].TryInteger()
	if err != nil {
		return 0, err
	}
	return bi.Int64(), nil
}

// topPublicKeysFromStack returns the top array of public keys from stack.
func topPublicKeysFromStack(st []stackitem.Item) (keys.PublicKeys, error) {
	index := len(st) - 1 // top stack element is last in the array
	var (
		pks keys.PublicKeys
		err error
	)
	items, ok := st[index].Value().([]stackitem.Item)
	if !ok {
		return nil, fmt.Errorf("invalid stack item type: %s", st[index].Type())
	}
	pks = make(keys.PublicKeys, len(items))
	for i, item := range items {
		val, ok := item.Value().([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid array element #%d: %s", i, item.Type())
		}
		pks[i], err = keys.NewPublicKeyFromBytes(val, elliptic.P256())
		if err != nil {
			return nil, err
		}
	}
	return pks, nil
}

// top string from stack returns the top string from stack.
func topStringFromStack(st []stackitem.Item) (string, error) {
	index := len(st) - 1 // top stack element is last in the array
	bs, err := st[index].TryBytes()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// topUint160FromStack returns the top util.Uint160 from stack.
func topUint160FromStack(st []stackitem.Item) (util.Uint160, error) {
	index := len(st) - 1 // top stack element is last in the array
	bs, err := st[index].TryBytes()
	if err != nil {
		return util.Uint160{}, err
	}
	return util.Uint160DecodeBytesBE(bs)
}

// topMapFromStack returns the top stackitem.Map from stack.
func topMapFromStack(st []stackitem.Item) (*stackitem.Map, error) {
	index := len(st) - 1 // top stack element is last in the array
	if t := st[index].Type(); t != stackitem.MapT {
		return nil, fmt.Errorf("invalid return stackitem type: %s", t.String())
	}
	return st[index].(*stackitem.Map), nil
}

// topIterableFromStack returns top list of elements of `resultItemType` type from stack.
func topIterableFromStack(st []stackitem.Item, resultItemType interface{}) ([]interface{}, error) {
	index := len(st) - 1 // top stack element is last in the array
	if t := st[index].Type(); t != stackitem.InteropT {
		return nil, fmt.Errorf("invalid return stackitem type: %s (InteropInterface expected)", t.String())
	}
	iter, ok := st[index].Value().(result.Iterator)
	if !ok {
		return nil, fmt.Errorf("failed to deserialize iterable from interop stackitem: invalid value type (Array expected)")
	}
	result := make([]interface{}, len(iter.Values))
	for i := range iter.Values {
		switch resultItemType.(type) {
		case string:
			bytes, err := iter.Values[i].TryBytes()
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize string from stackitem #%d: %w", i, err)
			}
			result[i] = string(bytes)
		case util.Uint160:
			bytes, err := iter.Values[i].TryBytes()
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize uint160 from stackitem #%d: %w", i, err)
			}
			result[i], err = util.Uint160DecodeBytesBE(bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to decode uint160 from stackitem #%d: %w", i, err)
			}
		default:
			return nil, errors.New("unsupported iterable type")
		}
	}
	return result, nil
}
