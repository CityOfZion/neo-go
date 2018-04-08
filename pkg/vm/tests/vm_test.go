package vm_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name   string
	src    string
	result interface{}
}

func eval(t *testing.T, src string, result interface{}) {
	vm := vmAndCompile(t, src)
	vm.Run()
	assertResult(t, vm, result)
}

func evalWithArgs(t *testing.T, src string, op []byte, args []vm.StackItem, result interface{}) {
	vm := vmAndCompile(t, src)
	vm.LoadArgs(op, args)
	vm.Run()
	assertResult(t, vm, result)
}

func assertResult(t *testing.T, vm *vm.VM, result interface{}) {
	assert.Equal(t, result, vm.PopResult())
	assert.Equal(t, 0, vm.Astack().Len())
	assert.Equal(t, 0, vm.Istack().Len())
}

func vmAndCompile(t *testing.T, src string) *vm.VM {
	vm := vm.New(vm.ModeMute)

	storePlugin := newStoragePlugin()
	vm.RegisterInteropFunc("Neo.Storage.Get", storePlugin.Get)
	vm.RegisterInteropFunc("Neo.Storage.Put", storePlugin.Put)

	b, err := compiler.Compile(strings.NewReader(src), &compiler.Options{})
	if err != nil {
		t.Fatal(err)
	}
	vm.Load(b)
	return vm
}

func TestVMAndCompilerCases(t *testing.T) {
	vm := vm.New(vm.ModeMute)

	storePlugin := newStoragePlugin()
	vm.RegisterInteropFunc("Neo.Storage.Get", storePlugin.Get)

	testCases := []testCase{}
	testCases = append(testCases, numericTestCases...)
	testCases = append(testCases, assignTestCases...)
	testCases = append(testCases, binaryExprTestCases...)
	testCases = append(testCases, structTestCases...)

	for _, tc := range testCases {
		b, err := compiler.Compile(strings.NewReader(tc.src), &compiler.Options{})
		if err != nil {
			t.Fatal(err)
		}
		vm.Load(b)
		vm.Run()
		assert.Equal(t, tc.result, vm.PopResult())
	}
}

type storagePlugin struct {
	mem map[string][]byte
}

func newStoragePlugin() *storagePlugin {
	return &storagePlugin{
		mem: make(map[string][]byte),
	}
}

func (s *storagePlugin) Delete(vm *vm.VM) error {
	vm.Estack().Pop()
	key := vm.Estack().Pop().Bytes()
	delete(s.mem, string(key))
	return nil
}

func (s *storagePlugin) Put(vm *vm.VM) error {
	vm.Estack().Pop()
	key := vm.Estack().Pop().Bytes()
	value := vm.Estack().Pop().Bytes()
	s.mem[string(key)] = value
	return nil
}

func (s *storagePlugin) Get(vm *vm.VM) error {
	vm.Estack().Pop()
	item := vm.Estack().Pop().Bytes()
	if val, ok := s.mem[string(item)]; ok {
		vm.Estack().PushVal(val)
		return nil
	}
	return fmt.Errorf("could not find %+v", item)
}
