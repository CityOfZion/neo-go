package compiler

import (
	"go/ast"
	"go/constant"
	"go/types"
	"log"

	"golang.org/x/tools/go/loader"
)

// typeAndValueForField returns a zero initialized typeAndValue for the given type.Var.
func typeAndValueForField(fld *types.Var) types.TypeAndValue {
	switch t := fld.Type().(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Int:
			return types.TypeAndValue{
				Type:  t,
				Value: constant.MakeInt64(0),
			}
		case types.String:
			return types.TypeAndValue{
				Type:  t,
				Value: constant.MakeString(""),
			}
		case types.Bool, types.UntypedBool:
			return types.TypeAndValue{
				Type:  t,
				Value: constant.MakeBool(false),
			}
		default:
			log.Fatalf("could not initialize struct field %s to zero, type: %s", fld.Name(), t)
		}
	}
	return types.TypeAndValue{}
}

// countGlobals counts the global variables in the program to add
// them with the stacksize of the function.
func countGlobals(f *ast.File) (i int64) {
	ast.Inspect(f, func(node ast.Node) bool {
		switch node.(type) {
		// Skip all functio declarations.
		case *ast.FuncDecl:
			return false
		// After skipping all funcDecls we are sure that each value spec
		// is a global declared variable or constant.
		case *ast.ValueSpec:
			i++
		}
		return true
	})
	return
}

// isIdentBool looks if the given ident is a boolean.
func isIdentBool(ident *ast.Ident) bool {
	return ident.Name == "true" || ident.Name == "false"
}

// makeBoolFromIdent creates a bool type from an *ast.Ident.
func makeBoolFromIdent(ident *ast.Ident, tinfo *types.Info) types.TypeAndValue {
	var b bool
	if ident.Name == "true" {
		b = true
	} else if ident.Name == "false" {
		b = false
	} else {
		log.Fatalf("givent identifier cannot be converted to a boolean => %s", ident.Name)
	}
	return types.TypeAndValue{
		Type:  tinfo.ObjectOf(ident).Type(),
		Value: constant.MakeBool(b),
	}
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

// indexOfStruct will return the index of the given field inside that struct.
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

func isByteArray(lit *ast.CompositeLit, tInfo *types.Info) bool {
	if len(lit.Elts) == 0 {
		return false
	}

	typ := tInfo.Types[lit.Elts[0]].Type.Underlying()
	switch t := typ.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Byte:
			return true
		}
	}
	return false
}
