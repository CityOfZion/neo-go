package compiler

import (
	"errors"
	"go/ast"
	"go/types"

	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"golang.org/x/tools/go/loader"
)

var (
	// Go language builtin functions and custom builtin utility functions.
	builtinFuncs = []string{
		"len", "append", "SHA256",
		"AppCall",
		"FromAddress", "Equals",
		"panic",
		"ToBool", "ToByteArray", "ToInteger",
	}
)

// newGlobal creates new global variable.
func (c *codegen) newGlobal(name string) {
	c.globals[name] = len(c.globals)
}

// traverseGlobals visits and initializes global variables.
func (c *codegen) traverseGlobals(f ast.Node) {
	n := countGlobals(f)
	if n != 0 {
		if n > 255 {
			c.prog.BinWriter.Err = errors.New("too many global variables")
			return
		}
		emit.Instruction(c.prog.BinWriter, opcode.INITSSLOT, []byte{byte(n)})
	}
	c.convertGlobals(f)
}

// countGlobals counts the global variables in the program to add
// them with the stack size of the function.
func countGlobals(f ast.Node) (i int) {
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		// Skip all function declarations.
		case *ast.FuncDecl:
			return false
		// After skipping all funcDecls we are sure that each value spec
		// is a global declared variable or constant.
		case *ast.ValueSpec:
			i += len(n.Names)
		}
		return true
	})
	return
}

// isExprNil looks if the given expression is a `nil`.
func isExprNil(e ast.Expr) bool {
	v, ok := e.(*ast.Ident)
	return ok && v.Name == "nil"
}

// resolveEntryPoint returns the function declaration of the entrypoint and the corresponding file.
func resolveEntryPoint(entry string, pkg *loader.PackageInfo) (*ast.FuncDecl, *ast.File) {
	var (
		main *ast.FuncDecl
		file *ast.File
	)
	for _, f := range pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.FuncDecl:
				if t.Name.Name == entry {
					main = t
					file = f
					return false
				}
			}
			return true
		})
	}
	return main, file
}

// indexOfStruct returns the index of the given field inside that struct.
// If the struct does not contain that field it will return -1.
func indexOfStruct(strct *types.Struct, fldName string) int {
	for i := 0; i < strct.NumFields(); i++ {
		if strct.Field(i).Name() == fldName {
			return i
		}
	}
	return -1
}

type funcUsage map[string]bool

func (f funcUsage) funcUsed(name string) bool {
	_, ok := f[name]
	return ok
}

// lastStmtIsReturn checks if last statement of the declaration was return statement..
func lastStmtIsReturn(decl *ast.FuncDecl) (b bool) {
	if l := len(decl.Body.List); l != 0 {
		_, ok := decl.Body.List[l-1].(*ast.ReturnStmt)
		return ok
	}
	return false
}

func analyzeFuncUsage(pkgs map[*types.Package]*loader.PackageInfo) funcUsage {
	usage := funcUsage{}

	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.CallExpr:
					switch t := n.Fun.(type) {
					case *ast.Ident:
						usage[t.Name] = true
					case *ast.SelectorExpr:
						usage[t.Sel.Name] = true
					}
				}
				return true
			})
		}
	}
	return usage
}

func isBuiltin(expr ast.Expr) bool {
	var name string

	switch t := expr.(type) {
	case *ast.Ident:
		name = t.Name
	case *ast.SelectorExpr:
		name = t.Sel.Name
	default:
		return false
	}

	for _, n := range builtinFuncs {
		if name == n {
			return true
		}
	}
	return false
}

func isSyscall(fun *funcScope) bool {
	if fun.selector == nil {
		return false
	}
	_, ok := syscalls[fun.selector.Name][fun.name]
	return ok
}
