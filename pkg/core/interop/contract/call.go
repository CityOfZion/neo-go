package contract

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Call calls a contract.
func Call(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	method := ic.VM.Estack().Pop().String()
	args := ic.VM.Estack().Pop().Array()
	return callExInternal(ic, h, method, args, smartcontract.All)
}

// CallEx calls a contract with flags.
func CallEx(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	method := ic.VM.Estack().Pop().String()
	args := ic.VM.Estack().Pop().Array()
	flags := smartcontract.CallFlag(int32(ic.VM.Estack().Pop().BigInt().Int64()))
	if flags&^smartcontract.All != 0 {
		return errors.New("call flags out of range")
	}
	return callExInternal(ic, h, method, args, flags)
}

func callExInternal(ic *interop.Context, h []byte, name string, args []stackitem.Item, f smartcontract.CallFlag) error {
	u, err := util.Uint160DecodeBytesBE(h)
	if err != nil {
		return errors.New("invalid contract hash")
	}
	cs, err := ic.DAO.GetContractState(u)
	if err != nil {
		return errors.New("contract not found")
	}
	if strings.HasPrefix(name, "_") {
		return errors.New("invalid method name (starts with '_')")
	}
	curr, err := ic.DAO.GetContractState(ic.VM.GetCurrentScriptHash())
	if err == nil {
		if !curr.Manifest.CanCall(&cs.Manifest, name) {
			return errors.New("disallowed method call")
		}
	}
	return CallExInternal(ic, cs, name, args, f, vm.EnsureNotEmpty)
}

// CallExInternal calls a contract with flags and can't be invoked directly by user.
func CallExInternal(ic *interop.Context, cs *state.Contract,
	name string, args []stackitem.Item, f smartcontract.CallFlag, checkReturn vm.CheckReturnState) error {
	md := cs.Manifest.ABI.GetMethod(name)
	if md == nil {
		return fmt.Errorf("method '%s' not found", name)
	}

	if len(args) != len(md.Parameters) {
		return fmt.Errorf("invalid argument count: %d (expected %d)", len(args), len(md.Parameters))
	}

	u := cs.ScriptHash()
	ic.VM.Invocations[u]++
	ic.VM.LoadScriptWithHash(cs.Script, u, ic.VM.Context().GetCallFlags()&f)
	var isNative bool
	for i := range ic.Natives {
		if ic.Natives[i].Metadata().Hash.Equals(u) {
			isNative = true
			break
		}
	}
	if isNative {
		ic.VM.Estack().PushVal(args)
		ic.VM.Estack().PushVal(name)
	} else {
		for i := len(args) - 1; i >= 0; i-- {
			ic.VM.Estack().PushVal(args[i])
		}
		// use Jump not Call here because context was loaded in LoadScript above.
		ic.VM.Jump(ic.VM.Context(), md.Offset)
	}
	ic.VM.Context().CheckReturn = checkReturn

	md = cs.Manifest.ABI.GetMethod(manifest.MethodInit)
	if md != nil {
		ic.VM.Call(ic.VM.Context(), md.Offset)
	}

	return nil
}
