package runtime

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// GasLeft returns remaining amount of GAS.
func GasLeft(ic *interop.Context) error {
	if ic.VM.GasLimit == -1 {
		ic.VM.Estack().PushVal(ic.VM.GasLimit)
	} else {
		ic.VM.Estack().PushVal(ic.VM.GasLimit - ic.VM.GasConsumed())
	}
	return nil
}

// GetNotifications returns notifications emitted by current contract execution.
func GetNotifications(ic *interop.Context) error {
	item := ic.VM.Estack().Pop().Item()
	notifications := ic.Notifications
	if _, ok := item.(stackitem.Null); !ok {
		b, err := item.TryBytes()
		if err != nil {
			return err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return err
		}
		notifications = []state.NotificationEvent{}
		for i := range ic.Notifications {
			if ic.Notifications[i].ScriptHash.Equals(u) {
				notifications = append(notifications, ic.Notifications[i])
			}
		}
	}
	if len(notifications) > vm.MaxStackSize {
		return errors.New("too many notifications")
	}
	arr := stackitem.NewArray(make([]stackitem.Item, 0, len(notifications)))
	for i := range notifications {
		ev := stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(notifications[i].ScriptHash.BytesBE()),
			stackitem.Make(notifications[i].Name),
			stackitem.DeepCopy(notifications[i].Item).(*stackitem.Array),
		})
		arr.Append(ev)
	}
	ic.VM.Estack().PushVal(arr)
	return nil
}

// GetInvocationCounter returns how many times current contract was invoked during current tx execution.
func GetInvocationCounter(ic *interop.Context) error {
	count, ok := ic.Invocations[ic.VM.GetCurrentScriptHash()]
	if !ok {
		return errors.New("current contract wasn't invoked from others")
	}
	ic.VM.Estack().PushVal(count)
	return nil
}
