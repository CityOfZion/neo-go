package interop

import (
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/zap"
)

// Context represents context in which interops are executed.
type Context struct {
	Chain         blockchainer.Blockchainer
	Container     crypto.Verifiable
	Natives       []Contract
	Trigger       trigger.Type
	Block         *block.Block
	Tx            *transaction.Transaction
	DAO           *dao.Cached
	Notifications []state.NotificationEvent
	Log           *zap.Logger
	Invocations   map[util.Uint160]int
	VM            *vm.VM
	Functions     [][]Function
}

// NewContext returns new interop context.
func NewContext(trigger trigger.Type, bc blockchainer.Blockchainer, d dao.DAO, natives []Contract, block *block.Block, tx *transaction.Transaction, log *zap.Logger) *Context {
	dao := dao.NewCached(d)
	nes := make([]state.NotificationEvent, 0)
	return &Context{
		Chain:         bc,
		Natives:       natives,
		Trigger:       trigger,
		Block:         block,
		Tx:            tx,
		DAO:           dao,
		Notifications: nes,
		Log:           log,
		Invocations:   make(map[util.Uint160]int),
		// Functions is a slice of slices of interops sorted by ID.
		Functions: [][]Function{},
	}
}

// Function binds function name, id with the function itself and price,
// it's supposed to be inited once for all interopContexts, so it doesn't use
// vm.InteropFuncPrice directly.
type Function struct {
	ID   uint32
	Name string
	Func func(*Context) error
	// DisallowCallback is true iff syscall can't be used in a callback.
	DisallowCallback bool
	// ParamCount is a number of function parameters.
	ParamCount int
	Price      int64
	// RequiredFlags is a set of flags which must be set during script invocations.
	// Default value is NoneFlag i.e. no flags are required.
	RequiredFlags smartcontract.CallFlag
}

// Method is a signature for a native method.
type Method = func(ic *Context, args []stackitem.Item) stackitem.Item

// MethodAndPrice is a native-contract method descriptor.
type MethodAndPrice struct {
	Func          Method
	Price         int64
	RequiredFlags smartcontract.CallFlag
}

// Contract is an interface for all native contracts.
type Contract interface {
	Initialize(*Context) error
	Metadata() *ContractMD
}

// ContractMD represents native contract instance.
type ContractMD struct {
	Manifest   manifest.Manifest
	Name       string
	ContractID int32
	Script     []byte
	Hash       util.Uint160
	Methods    map[string]MethodAndPrice
}

// NewContractMD returns Contract with the specified list of methods.
func NewContractMD(name string) *ContractMD {
	c := &ContractMD{
		Name:    name,
		Methods: make(map[string]MethodAndPrice),
	}

	w := io.NewBufBinWriter()
	emit.String(w.BinWriter, c.Name)
	emit.Syscall(w.BinWriter, interopnames.NeoNativeCall)

	c.Script = w.Bytes()
	c.Hash = hash.Hash160(c.Script)
	c.Manifest = *manifest.DefaultManifest(c.Hash)

	return c
}

// AddMethod adds new method to a native contract.
func (c *ContractMD) AddMethod(md *MethodAndPrice, desc *manifest.Method, safe bool) {
	c.Manifest.ABI.Methods = append(c.Manifest.ABI.Methods, *desc)
	c.Methods[desc.Name] = *md
	if safe {
		c.Manifest.SafeMethods.Add(desc.Name)
	}
}

// AddEvent adds new event to a native contract.
func (c *ContractMD) AddEvent(name string, ps ...manifest.Parameter) {
	c.Manifest.ABI.Events = append(c.Manifest.ABI.Events, manifest.Event{
		Name:       name,
		Parameters: ps,
	})
}

// Sort sorts interop functions by id.
func Sort(fs []Function) {
	sort.Slice(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
}

// GetFunction returns metadata for interop with the specified id.
func (ic *Context) GetFunction(id uint32) *Function {
	for _, slice := range ic.Functions {
		n := sort.Search(len(slice), func(i int) bool {
			return slice[i].ID >= id
		})
		if n < len(slice) && slice[n].ID == id {
			return &slice[n]
		}
	}
	return nil
}

// SyscallHandler handles syscall with id.
func (ic *Context) SyscallHandler(_ *vm.VM, id uint32) error {
	f := ic.GetFunction(id)
	if f == nil {
		return errors.New("syscall not found")
	}
	cf := ic.VM.Context().GetCallFlags()
	if !cf.Has(f.RequiredFlags) {
		return fmt.Errorf("missing call flags: %05b vs %05b", cf, f.RequiredFlags)
	}
	if !ic.VM.AddGas(f.Price) {
		return errors.New("insufficient amount of gas")
	}
	return f.Func(ic)
}

// SpawnVM spawns new VM with the specified gas limit and set context.VM field.
func (ic *Context) SpawnVM() *vm.VM {
	v := vm.NewWithTrigger(ic.Trigger)
	v.GasLimit = -1
	v.SyscallHandler = ic.SyscallHandler
	ic.VM = v
	return v
}
