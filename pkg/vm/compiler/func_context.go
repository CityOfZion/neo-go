package compiler

import (
	"log"
)

// A FuncContext represents details about a function in the program along withs its variables.
type FuncContext struct {
	// Identifier.
	name string
	// The scope of the function.
	scope map[string]*VarContext
	// Address (label) where the compiler can find this function
	// when someone calls it.
	label int16
	// Counter for local variables.
	i int

	currentCtx *VarContext
}

func newFuncContext(name string) *FuncContext {
	return &FuncContext{
		name:  name,
		scope: map[string]*VarContext{},
	}
}

func (f *FuncContext) registerContext(name string) {
	if f.currentCtx == nil {
		log.Fatalf("The is no current context to register with name %s", name)
	}
	f.currentCtx.pos = f.i
	f.scope[f.currentCtx.name] = f.currentCtx
	f.i++
}

func (f *FuncContext) getContext(name string) *VarContext {
	ctx, ok := f.scope[name]
	if !ok {
		log.Fatalf("could not resolve variable %s", name)
	}
	return ctx
}

func (f *FuncContext) isRegistered(ctx *VarContext) bool {
	_, ok := f.scope[ctx.name]
	return ok
}
