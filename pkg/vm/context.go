package vm

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Context represents the current execution context of the VM.
type Context struct {
	// Instruction pointer.
	ip int

	// The next instruction pointer.
	nextip int

	// The raw program script.
	prog []byte

	// Breakpoints.
	breakPoints []int

	// Evaluation stack pointer.
	estack *Stack

	static    *Slot
	local     *Slot
	arguments *Slot

	// Exception context stack pointer.
	tryStack *Stack

	// Script hash of the prog.
	scriptHash util.Uint160

	// Caller's contract script hash.
	callingScriptHash util.Uint160

	// Call flags this context was created with.
	callFlag smartcontract.CallFlag

	// CheckReturn specifies if amount of return values needs to be checked.
	CheckReturn bool
}

var errNoInstParam = errors.New("failed to read instruction parameter")

// NewContext returns a new Context object.
func NewContext(b []byte) *Context {
	return &Context{
		prog:        b,
		breakPoints: []int{},
	}
}

// NextIP returns next instruction pointer.
func (c *Context) NextIP() int {
	return c.nextip
}

// Next returns the next instruction to execute with its parameter if any. After
// its invocation the instruction pointer points to the instruction being
// returned.
func (c *Context) Next() (opcode.Opcode, []byte, error) {
	var err error

	c.ip = c.nextip
	if c.ip >= len(c.prog) {
		return opcode.RET, nil, nil
	}

	var instrbyte = c.prog[c.ip]
	instr := opcode.Opcode(instrbyte)
	c.nextip++

	var numtoread int
	switch instr {
	case opcode.PUSHDATA1:
		if c.nextip >= len(c.prog) {
			err = errNoInstParam
		} else {
			numtoread = int(c.prog[c.nextip])
			c.nextip++
		}
	case opcode.PUSHDATA2:
		if c.nextip+1 >= len(c.prog) {
			err = errNoInstParam
		} else {
			numtoread = int(binary.LittleEndian.Uint16(c.prog[c.nextip : c.nextip+2]))
			c.nextip += 2
		}
	case opcode.PUSHDATA4:
		if c.nextip+3 >= len(c.prog) {
			err = errNoInstParam
		} else {
			var n = binary.LittleEndian.Uint32(c.prog[c.nextip : c.nextip+4])
			if n > stackitem.MaxSize {
				return instr, nil, errors.New("parameter is too big")
			}
			numtoread = int(n)
			c.nextip += 4
		}
	case opcode.JMP, opcode.JMPIF, opcode.JMPIFNOT, opcode.JMPEQ, opcode.JMPNE,
		opcode.JMPGT, opcode.JMPGE, opcode.JMPLT, opcode.JMPLE,
		opcode.CALL, opcode.ISTYPE, opcode.CONVERT, opcode.NEWARRAYT,
		opcode.ENDTRY,
		opcode.INITSSLOT, opcode.LDSFLD, opcode.STSFLD, opcode.LDARG, opcode.STARG, opcode.LDLOC, opcode.STLOC:
		numtoread = 1
	case opcode.INITSLOT, opcode.TRY:
		numtoread = 2
	case opcode.JMPL, opcode.JMPIFL, opcode.JMPIFNOTL, opcode.JMPEQL, opcode.JMPNEL,
		opcode.JMPGTL, opcode.JMPGEL, opcode.JMPLTL, opcode.JMPLEL,
		opcode.ENDTRYL,
		opcode.CALLL, opcode.SYSCALL, opcode.PUSHA:
		numtoread = 4
	case opcode.TRYL:
		numtoread = 8
	default:
		if instr <= opcode.PUSHINT256 {
			numtoread = 1 << instr
		} else {
			// No parameters, can just return.
			return instr, nil, nil
		}
	}
	if c.nextip+numtoread-1 >= len(c.prog) {
		err = errNoInstParam
	}
	if err != nil {
		return instr, nil, err
	}
	parameter := make([]byte, numtoread)
	copy(parameter, c.prog[c.nextip:c.nextip+numtoread])
	c.nextip += numtoread
	return instr, parameter, nil
}

// IP returns current instruction offset in the context script.
func (c *Context) IP() int {
	return c.ip
}

// LenInstr returns the number of instructions loaded.
func (c *Context) LenInstr() int {
	return len(c.prog)
}

// CurrInstr returns the current instruction and opcode.
func (c *Context) CurrInstr() (int, opcode.Opcode) {
	return c.ip, opcode.Opcode(c.prog[c.ip])
}

// Copy returns an new exact copy of c.
func (c *Context) Copy() *Context {
	ctx := new(Context)
	*ctx = *c
	return ctx
}

// GetCallFlags returns calling flags context was created with.
func (c *Context) GetCallFlags() smartcontract.CallFlag {
	return c.callFlag
}

// Program returns the loaded program.
func (c *Context) Program() []byte {
	return c.prog
}

// ScriptHash returns a hash of the script in the current context.
func (c *Context) ScriptHash() util.Uint160 {
	if c.scriptHash.Equals(util.Uint160{}) {
		c.scriptHash = hash.Hash160(c.prog)
	}
	return c.scriptHash
}

// Value implements stackitem.Item interface.
func (c *Context) Value() interface{} {
	return c
}

// Dup implements stackitem.Item interface.
func (c *Context) Dup() stackitem.Item {
	return c
}

// TryBool implements stackitem.Item interface.
func (c *Context) TryBool() (bool, error) { panic("can't convert Context to Bool") }

// TryBytes implements stackitem.Item interface.
func (c *Context) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Context to ByteArray")
}

// TryInteger implements stackitem.Item interface.
func (c *Context) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Context to Integer")
}

// Type implements stackitem.Item interface.
func (c *Context) Type() stackitem.Type { panic("Context cannot appear on evaluation stack") }

// Convert implements stackitem.Item interface.
func (c *Context) Convert(_ stackitem.Type) (stackitem.Item, error) {
	panic("Context cannot be converted to anything")
}

// Equals implements stackitem.Item interface.
func (c *Context) Equals(s stackitem.Item) bool {
	return c == s
}

func (c *Context) atBreakPoint() bool {
	for _, n := range c.breakPoints {
		if n == c.nextip {
			return true
		}
	}
	return false
}

func (c *Context) String() string {
	return "execution context"
}

// getContextScriptHash returns script hash of the invocation stack element
// number n.
func (v *VM) getContextScriptHash(n int) util.Uint160 {
	element := v.Istack().Peek(n)
	if element == nil {
		return util.Uint160{}
	}
	ctxIface := element.Value()
	ctx := ctxIface.(*Context)
	return ctx.ScriptHash()
}

// PushContextScriptHash pushes to evaluation stack the script hash of the
// invocation stack element number n.
func (v *VM) PushContextScriptHash(n int) error {
	h := v.getContextScriptHash(n)
	v.Estack().PushVal(h.BytesBE())
	return nil
}
