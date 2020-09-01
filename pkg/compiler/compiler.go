package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"golang.org/x/tools/go/loader"
)

const fileExt = "nef"

// Options contains all the parameters that affect the behaviour of the compiler.
type Options struct {
	// The extension of the output file default set to .nef
	Ext string

	// The name of the output file.
	Outfile string

	// The name of the output for debug info.
	DebugInfo string

	// The name of the output for contract manifest file.
	ManifestFile string

	// Contract features.
	ContractFeatures smartcontract.PropertyState

	// Runtime notifications.
	ContractEvents []manifest.Event

	// The list of standards supported by the contract.
	ContractSupportedStandards []string
}

type buildInfo struct {
	initialPackage string
	program        *loader.Program
}

// ForEachPackage executes fn on each package used in the current program
// in the order they should be initialized.
func (c *codegen) ForEachPackage(fn func(*loader.PackageInfo)) {
	for i := range c.packages {
		pkg := c.buildInfo.program.Package(c.packages[i])
		c.typeInfo = &pkg.Info
		c.currPkg = pkg.Pkg
		fn(pkg)
	}
}

// ForEachFile executes fn on each file used in current program.
func (c *codegen) ForEachFile(fn func(*ast.File, *types.Package)) {
	c.ForEachPackage(func(pkg *loader.PackageInfo) {
		for _, f := range pkg.Files {
			c.fillImportMap(f, pkg.Pkg)
			fn(f, pkg.Pkg)
		}
	})
}

// fillImportMap fills import map for f.
func (c *codegen) fillImportMap(f *ast.File, pkg *types.Package) {
	c.importMap = map[string]string{"": pkg.Path()}
	for _, imp := range f.Imports {
		// We need to load find package metadata because
		// name specified in `package ...` decl, can be in
		// conflict with package path.
		pkgPath := strings.Trim(imp.Path.Value, `"`)
		realPkg := c.buildInfo.program.Package(pkgPath)
		name := realPkg.Pkg.Name()
		if imp.Name != nil {
			name = imp.Name.Name
		}
		c.importMap[name] = realPkg.Pkg.Path()
	}
}

func getBuildInfo(name string, src interface{}) (*buildInfo, error) {
	conf := loader.Config{ParserMode: parser.ParseComments}
	if src != nil {
		f, err := conf.ParseFile(name, src)
		if err != nil {
			return nil, err
		}
		conf.CreateFromFiles("", f)
	} else {
		var names []string
		if strings.HasSuffix(name, ".go") {
			names = append(names, name)
		} else {
			ds, err := ioutil.ReadDir(name)
			if err != nil {
				return nil, fmt.Errorf("'%s' is neither Go source nor a directory", name)
			}
			for i := range ds {
				if !ds[i].IsDir() && strings.HasSuffix(ds[i].Name(), ".go") {
					names = append(names, path.Join(name, ds[i].Name()))
				}
			}
		}
		if len(names) == 0 {
			return nil, errors.New("no files provided")
		}
		conf.CreateFromFilenames("", names...)
	}

	prog, err := conf.Load()
	if err != nil {
		return nil, err
	}

	return &buildInfo{
		initialPackage: prog.InitialPackages()[0].Pkg.Name(),
		program:        prog,
	}, nil
}

// Compile compiles a Go program into bytecode that can run on the NEO virtual machine.
// If `r != nil`, `name` is interpreted as a filename, and `r` as file contents.
// Otherwise `name` is either file name or name of the directory containing source files.
func Compile(name string, r io.Reader) ([]byte, error) {
	buf, _, err := CompileWithDebugInfo(name, r)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// CompileWithDebugInfo compiles a Go program into bytecode and emits debug info.
func CompileWithDebugInfo(name string, r io.Reader) ([]byte, *DebugInfo, error) {
	ctx, err := getBuildInfo(name, r)
	if err != nil {
		return nil, nil, err
	}
	return CodeGen(ctx)
}

// CompileAndSave will compile and save the file to disk in the NEF format.
func CompileAndSave(src string, o *Options) ([]byte, error) {
	o.Outfile = strings.TrimSuffix(o.Outfile, fmt.Sprintf(".%s", fileExt))
	if len(o.Outfile) == 0 {
		if strings.HasSuffix(src, ".go") {
			o.Outfile = strings.TrimSuffix(src, ".go")
		} else {
			o.Outfile = "out"
		}
	}
	if len(o.Ext) == 0 {
		o.Ext = fileExt
	}
	b, di, err := CompileWithDebugInfo(src, nil)
	if err != nil {
		return nil, fmt.Errorf("error while trying to compile smart contract file: %w", err)
	}
	f, err := nef.NewFile(b)
	if err != nil {
		return nil, fmt.Errorf("error while trying to create .nef file: %w", err)
	}
	bytes, err := f.Bytes()
	if err != nil {
		return nil, fmt.Errorf("error while serializing .nef file: %w", err)
	}
	out := fmt.Sprintf("%s.%s", o.Outfile, o.Ext)
	err = ioutil.WriteFile(out, bytes, os.ModePerm)
	if err != nil {
		return b, err
	}
	if o.DebugInfo == "" && o.ManifestFile == "" {
		return b, nil
	}

	if o.DebugInfo != "" {
		di.Events = make([]EventDebugInfo, len(o.ContractEvents))
		for i, e := range o.ContractEvents {
			params := make([]DebugParam, len(e.Parameters))
			for j, p := range e.Parameters {
				params[j] = DebugParam{
					Name: p.Name,
					Type: p.Type.String(),
				}
			}
			di.Events[i] = EventDebugInfo{
				ID: e.Name,
				// DebugInfo event name should be at the format {namespace},{name}
				// but we don't provide namespace via .yml config
				Name:       "," + e.Name,
				Parameters: params,
			}
		}
		data, err := json.Marshal(di)
		if err != nil {
			return b, err
		}
		if err := ioutil.WriteFile(o.DebugInfo, data, os.ModePerm); err != nil {
			return b, err
		}
	}

	if o.ManifestFile != "" {
		m, err := di.ConvertToManifest(o.ContractFeatures, o.ContractEvents, o.ContractSupportedStandards...)
		if err != nil {
			return b, fmt.Errorf("failed to convert debug info to manifest: %w", err)
		}
		mData, err := json.Marshal(m)
		if err != nil {
			return b, fmt.Errorf("failed to marshal manifest to JSON: %w", err)
		}
		return b, ioutil.WriteFile(o.ManifestFile, mData, os.ModePerm)
	}

	return b, nil
}
