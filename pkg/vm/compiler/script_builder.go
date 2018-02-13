package compiler

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// ScriptBuilder generates bytecode and will write all
// generated bytecode into its internal buffer.
type ScriptBuilder struct {
	buf *bytes.Buffer
}

func (sb *ScriptBuilder) emit(op vm.OpCode, b []byte) error {
	if err := sb.buf.WriteByte(byte(op)); err != nil {
		return err
	}
	_, err := sb.buf.Write(b)
	return err
}

func (sb *ScriptBuilder) emitPush(op vm.OpCode) error {
	return sb.buf.WriteByte(byte(op))
}

func (sb *ScriptBuilder) emitPushBool(b bool) error {
	if b {
		return sb.emitPush(vm.OpPushT)
	}
	return sb.emitPush(vm.OpPushF)
}

func (sb *ScriptBuilder) emitPushInt(i int64) error {
	if i == -1 {
		return sb.emitPush(vm.OpPushM1)
	}
	if i == 0 {
		return sb.emitPush(vm.OpPushF)
	}
	if i > 0 && i < 16 {
		val := vm.OpCode((int(vm.OpPush1) - 1 + int(i)))
		return sb.emitPush(val)
	}

	bInt := big.NewInt(i)
	val := util.ToArrayReverse(bInt.Bytes())
	return sb.emitPushArray(val)
}

func (sb *ScriptBuilder) emitPushArray(b []byte) error {
	var (
		err error
		n   = len(b)
	)

	if n == 0 {
		return errors.New("0 bytes given in pushArray")
	}
	if n <= int(vm.OpPushBytes75) {
		return sb.emit(vm.OpCode(n), b)
	} else if n < 0x100 {
		err = sb.emit(vm.OpPushData1, []byte{byte(n)})
	} else if n < 0x10000 {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		err = sb.emit(vm.OpPushData2, buf)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		err = sb.emit(vm.OpPushData4, buf)
	}
	if err != nil {
		return err
	}
	_, err = sb.buf.Write(b)
	return err
}

func (sb *ScriptBuilder) emitPushString(str string) error {
	return sb.emitPushArray([]byte(str))
}

func (sb *ScriptBuilder) emitSysCall(api string) error {
	lenAPI := len(api)
	if lenAPI == 0 {
		return errors.New("syscall argument cant be 0")
	}
	if lenAPI > 252 {
		return fmt.Errorf("invalid syscall argument: %s", api)
	}

	bapi := []byte(api)
	args := make([]byte, lenAPI+1)
	args[0] = byte(lenAPI)
	copy(args, bapi[1:])
	return sb.emit(vm.OpSysCall, args)
}

func (sb *ScriptBuilder) emitPushCall(offset int16) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, offset)
	return sb.emit(vm.OpCall, buf.Bytes())
}

func (sb *ScriptBuilder) emitJump(op vm.OpCode, offset int16) error {
	if op != vm.OpJMP && op != vm.OpJMPIF && op != vm.OpJMPIFNOT && op != vm.OpCall {
		return fmt.Errorf("invalid jump opcode: %v", op)
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(offset))
	return sb.emit(op, buf) // convert to bits?
}

func (sb *ScriptBuilder) updateJmpLabel(label int16, offset int) error {
	sizeOfInt16 := 2
	if sizeOfInt16+offset >= sb.buf.Len() {
		return fmt.Errorf("cannot update label at offset %d", offset)
	}

	b := make([]byte, sizeOfInt16)
	binary.LittleEndian.PutUint16(b, uint16(label))
	buf := sb.buf.Bytes()
	copy(buf[offset:offset+sizeOfInt16], b)
	return nil
}

func (sb *ScriptBuilder) updatePushCall(offset int, label int16) {
	b := new(bytes.Buffer)
	binary.Write(b, binary.LittleEndian, label)

	buf := sb.buf.Bytes()
	copy(buf[offset:offset+2], b.Bytes())
}

func (sb *ScriptBuilder) dumpOpcode() {
	buf := sb.buf.Bytes()
	for i := 0; i < len(buf); i++ {
		fmt.Printf("OPCODE AT INDEX \t %d \t 0x%2x \t %s \n", i, buf[i], vm.OpCode(buf[i]))
	}
}
