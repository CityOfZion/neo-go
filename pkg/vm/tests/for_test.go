package vm_test

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm"
)

func TestArrayFieldInStruct(t *testing.T) {
	src := `
		package foo

		type Bar struct {
			arr []int
		}

		func Main() int {
			b := Bar{
				arr: []int{0, 1, 2},
			}
			x := b.arr[2]
			return x + 2
		}
	`
	eval(t, src, big.NewInt(4))
}

func TestArrayItemGetIndexBinaryExpr(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x + 1]
		}
	`
	eval(t, src, big.NewInt(2))
}

func TestArrayItemGetIndexIdent(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x]
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestArrayItemBinExpr(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			return x[1] + 10
		}
	`
	eval(t, src, big.NewInt(11))
}

func TestArrayItemReturn(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{0, 1, 2}
			return arr[1]
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestArrayItemAssign(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{1, 2, 3}
			y := x[0]
			return y
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestStringArray(t *testing.T) {
	src := `
		package foo
		func Main() []string {
			x := []string{"foo", "bar", "foobar"}
			return x
		}
	`
	eval(t, src, []vm.StackItem{
		vm.NewByteArrayItem([]byte("foo")),
		vm.NewByteArrayItem([]byte("bar")),
		vm.NewByteArrayItem([]byte("foobar")),
	})
}

func TestIntArray(t *testing.T) {
	src := `
		package foo
		func Main() []int {
			arr := []int{1, 2, 3}
			return arr
		}
	`
	eval(t, src, []vm.StackItem{
		vm.NewBigIntegerItem(1),
		vm.NewBigIntegerItem(2),
		vm.NewBigIntegerItem(3),
	})
}

func TestArrayLen(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{0, 1, 2}
			return len(arr)
		}
	`
	eval(t, src, big.NewInt(3))
}

func TestStringLen(t *testing.T) {
	src := `
		package foo
		func Main() int {
			str := "this is medium sized string"
			return len(str)
		}
	`
	eval(t, src, big.NewInt(27))
}

func TestSimpleString(t *testing.T) {
	src := `
		package foo
		func Main() string {
			x := "NEO"
			return x
		}
	`
	eval(t, src, vm.NewByteArrayItem([]byte("NEO")).Value())
}

func TestBoolAssign(t *testing.T) {
	src := `
		package foo
		func Main() bool {
			x := true
			return x
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestBoolCompare(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if x {
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestBoolCompareVerbose(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if x == true {
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestBoolUnaryInvert(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if !x { 
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(0))
}

func TestAppendString(t *testing.T) {
	src := `
	package foo
		func Main() string {
			arr := []string{"a", "b", "c"}
			arr = append(arr, "d")
			return arr[3]
		}
	`
	eval(t, src, vm.NewByteArrayItem([]byte("d")).Value())
}

func TestAppendInt(t *testing.T) {
	src := `
	package foo
		func Main() int {
			arr := []int{0, 1, 2}
			arr = append(arr, 3)
			return arr[3]
		}
	`
	eval(t, src, big.NewInt(3))
}

func TestClassicForLoop(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 0
			for i := 0; i < 10; i++ {
				x = i
			}
			return x
		}
	`
	eval(t, src, big.NewInt(9))
}

func TestInc(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 0
			x++
			return x
		}
	`

	eval(t, src, big.NewInt(1))
}

func TestDec(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 2
			x--
			return x
		}
	`

	eval(t, src, big.NewInt(1))
}

// TODO: This could be a nasty bug. Output of the VM is 65695.
// Only happens above 100000, could be binary read issue.
//func TestForLoopBigIter(t *testing.T) {
//	src := `
//	package foo
//		func Main() int {
//			x := 0
//			for i := 0; i < 100000; i++ {
//				x = i
//			}
//			return x
//		}
//	`
//	eval(t, src, big.NewInt(99999))
//}
