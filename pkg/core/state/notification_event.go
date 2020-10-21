package state

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NotificationEvent is a tuple of scripthash that emitted the Item as a
// notification and that item itself.
type NotificationEvent struct {
	ScriptHash util.Uint160     `json:"contract"`
	Name       string           `json:"eventname"`
	Item       *stackitem.Array `json:"state"`
}

// AppExecResult represent the result of the script execution, gathering together
// all resulting notifications, state, stack and other metadata.
type AppExecResult struct {
	TxHash         util.Uint256
	Trigger        trigger.Type
	VMState        vm.State
	GasConsumed    int64
	Stack          []stackitem.Item
	Events         []NotificationEvent
	FaultException string
}

// EncodeBinary implements the Serializable interface.
func (ne *NotificationEvent) EncodeBinary(w *io.BinWriter) {
	ne.ScriptHash.EncodeBinary(w)
	w.WriteString(ne.Name)
	stackitem.EncodeBinaryStackItem(ne.Item, w)
}

// DecodeBinary implements the Serializable interface.
func (ne *NotificationEvent) DecodeBinary(r *io.BinReader) {
	ne.ScriptHash.DecodeBinary(r)
	ne.Name = r.ReadString()
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err != nil {
		return
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		r.Err = errors.New("Array or Struct expected")
		return
	}
	ne.Item = stackitem.NewArray(arr)
}

// EncodeBinary implements the Serializable interface.
func (aer *AppExecResult) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(aer.TxHash[:])
	w.WriteB(byte(aer.Trigger))
	w.WriteB(byte(aer.VMState))
	w.WriteU64LE(uint64(aer.GasConsumed))
	stackitem.EncodeBinaryStackItem(stackitem.NewArray(aer.Stack), w)
	w.WriteArray(aer.Events)
	w.WriteVarBytes([]byte(aer.FaultException))
}

// DecodeBinary implements the Serializable interface.
func (aer *AppExecResult) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(aer.TxHash[:])
	aer.Trigger = trigger.Type(r.ReadB())
	aer.VMState = vm.State(r.ReadB())
	aer.GasConsumed = int64(r.ReadU64LE())
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err == nil {
		arr, ok := item.Value().([]stackitem.Item)
		if !ok {
			r.Err = errors.New("array expected")
			return
		}
		aer.Stack = arr
	}
	r.ReadArray(&aer.Events)
	aer.FaultException = r.ReadString()
}

// notificationEventAux is an auxiliary struct for NotificationEvent JSON marshalling.
type notificationEventAux struct {
	ScriptHash util.Uint160    `json:"contract"`
	Name       string          `json:"eventname"`
	Item       json.RawMessage `json:"state"`
}

// MarshalJSON implements implements json.Marshaler interface.
func (ne NotificationEvent) MarshalJSON() ([]byte, error) {
	item, err := stackitem.ToJSONWithTypes(ne.Item)
	if err != nil {
		item = []byte(`"error: recursive reference"`)
	}
	return json.Marshal(&notificationEventAux{
		ScriptHash: ne.ScriptHash,
		Name:       ne.Name,
		Item:       item,
	})
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (ne *NotificationEvent) UnmarshalJSON(data []byte) error {
	aux := new(notificationEventAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	item, err := stackitem.FromJSONWithTypes(aux.Item)
	if err != nil {
		return err
	}
	if t := item.Type(); t != stackitem.ArrayT {
		return fmt.Errorf("failed to convert notification event state of type %s to array", t.String())
	}
	ne.Item = item.(*stackitem.Array)
	ne.Name = aux.Name
	ne.ScriptHash = aux.ScriptHash
	return nil
}

// appExecResultAux is an auxiliary struct for JSON marshalling
type appExecResultAux struct {
	TxHash         *util.Uint256       `json:"txid"`
	Trigger        string              `json:"trigger"`
	VMState        string              `json:"vmstate"`
	GasConsumed    int64               `json:"gasconsumed,string"`
	Stack          json.RawMessage     `json:"stack"`
	Events         []NotificationEvent `json:"notifications"`
	FaultException string              `json:"exception,omitempty"`
}

// MarshalJSON implements implements json.Marshaler interface.
func (aer *AppExecResult) MarshalJSON() ([]byte, error) {
	var st json.RawMessage
	arr := make([]json.RawMessage, len(aer.Stack))
	for i := range arr {
		data, err := stackitem.ToJSONWithTypes(aer.Stack[i])
		if err != nil {
			st = []byte(`"error: recursive reference"`)
			break
		}
		arr[i] = data
	}

	var err error
	if st == nil {
		st, err = json.Marshal(arr)
		if err != nil {
			return nil, err
		}
	}

	// do not marshal block hash
	var hash *util.Uint256
	if aer.Trigger == trigger.Application {
		hash = &aer.TxHash
	}
	return json.Marshal(&appExecResultAux{
		TxHash:         hash,
		Trigger:        aer.Trigger.String(),
		VMState:        aer.VMState.String(),
		GasConsumed:    aer.GasConsumed,
		Stack:          st,
		Events:         aer.Events,
		FaultException: aer.FaultException,
	})
}

// UnmarshalJSON implements implements json.Unmarshaler interface.
func (aer *AppExecResult) UnmarshalJSON(data []byte) error {
	aux := new(appExecResultAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(aux.Stack, &arr); err == nil {
		st := make([]stackitem.Item, len(arr))
		for i := range arr {
			st[i], err = stackitem.FromJSONWithTypes(arr[i])
			if err != nil {
				break
			}
		}
		if err == nil {
			aer.Stack = st
		}
	}

	trigger, err := trigger.FromString(aux.Trigger)
	if err != nil {
		return err
	}
	aer.Trigger = trigger
	if aux.TxHash != nil {
		aer.TxHash = *aux.TxHash
	}
	state, err := vm.StateFromString(aux.VMState)
	if err != nil {
		return err
	}
	aer.VMState = state
	aer.Events = aux.Events
	aer.GasConsumed = aux.GasConsumed
	aer.FaultException = aux.FaultException

	return nil
}
