package compiler

import (
	"fmt"
	"os"
	"testing"
	"text/tabwriter"

	"github.com/CityOfZion/neo-go/pkg/vm"
)

type testCase struct {
	name   string
	src    string
	result interface{}
}

func TestAllCases(t *testing.T) {
	// The Go language
	//testCases = append(testCases, builtinTestCases...)
	//testCases = append(testCases, arrayTestCases...)
	//testCases = append(testCases, binaryExprTestCases...)
	//testCases = append(testCases, functionCallTestCases...)
	//testCases = append(testCases, boolTestCases...)
	//testCases = append(testCases, stringTestCases...)
	//testCases = append(testCases, structTestCases...)
	//testCases = append(testCases, ifStatementTestCases...)
	//testCases = append(testCases, customTypeTestCases...)
	//testCases = append(testCases, constantTestCases...)
	//testCases = append(testCases, importTestCases...)
	//testCases = append(testCases, forTestCases...)

	//// Blockchain specific
	//testCases = append(testCases, storageTestCases...)
	//testCases = append(testCases, runtimeTestCases...)

	//for _, tc := range testCases {
	//	b, err := compiler.Compile(strings.NewReader(tc.src), &compiler.Options{})
	//	if err != nil {
	//		t.Fatal(err)
	//	}

	//	expectedResult, err := hex.DecodeString(tc.result)
	//	if err != nil {
	//		t.Fatal(err)
	//	}

	//	if bytes.Compare(b, expectedResult) != 0 {
	//		fmt.Println(tc.src)
	//		t.Log(hex.EncodeToString(b))
	//		dumpOpCodeSideBySide(b, expectedResult)
	//		t.Fatalf("compiling %s failed", tc.name)
	//	}
	//}
}

func dumpOpCodeSideBySide(have, want []byte) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "INDEX\tHAVE OPCODE\tDESC\tWANT OPCODE\tDESC\tDIFF")

	var b byte
	for i := 0; i < len(have); i++ {
		if len(want) <= i {
			b = 0x00
		} else {
			b = want[i]
		}
		diff := ""
		if have[i] != b {
			diff = "<<"
		}
		fmt.Fprintf(w, "%d\t0x%2x\t%s\t0x%2x\t%s\t%s\n",
			i, have[i], vm.Opcode(have[i]), b, vm.Opcode(b), diff)
	}
	w.Flush()
}
