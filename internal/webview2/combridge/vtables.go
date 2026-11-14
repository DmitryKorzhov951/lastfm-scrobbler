//go:build windows

package combridge

import (
	"fmt"
	"reflect"
	"sync"

	"golang.org/x/sys/windows"
)

var (
	vTablesL sync.Mutex
	vTables  = make(map[string]*vTable)
)

func RegisterVTable[TParent, T IUnknown](guid string, fns ...interface{}) {
	registerVTableInternal[TParent, T](guid, false, fns...)
}

type vTable struct {
	Parent *vTable

	Name      string
	ComGUID   string
	ComVTable uintptr
	ComProcs  []uintptr
}

func registerVTableInternal[TParent, T IUnknown](guid string, isInternal bool, fns ...interface{}) {
	vTablesL.Lock()
	defer vTablesL.Unlock()

	t, tName := typeInterfaceToString[T]()
	tParent, tParentName := typeInterfaceToString[TParent]()
	if !t.Implements(tParent) {
		panic(fmt.Errorf("RegisterVTable '%s': '%s' must implement '%s'", tName, tName, tParentName))
	}

	if !isInternal {
		if t == reflect.TypeOf((*IUnknown)(nil)).Elem() {
			panic(fmt.Errorf("RegisterVTable '%s' IUnknown can't be registered", tName))
		}

		if t == tParent {
			panic(fmt.Errorf("RegisterVTable '%s': T and TParent can't be the same type", tName))
		}
	}

	var parent *vTable
	var parentProcs []uintptr
	var parentProcsCount int
	if t != tParent {
		parent = vTables[tParentName]
		if parent == nil {
			panic(fmt.Errorf("RegisterVTable '%s': Parent VTable '%s' not registered", tName, tParentName))
		}

		parentProcs = parent.ComProcs
		parentProcsCount = len(parentProcs)
	}

	comGuid, err := windows.GUIDFromString(guid)
	if err != nil {
		panic(fmt.Errorf("RegisterVTable '%s': invalid guid: %s", tName, err))
	}

	vTable := &vTable{
		Parent:  parent,
		Name:    tName,
		ComGUID: comGuid.String(),
	}
	vTable.ComVTable, vTable.ComProcs = allocUintptrObject(parentProcsCount + len(fns))

	for i, proc := range parentProcs {
		vTable.ComProcs[i] = proc
	}

	for i, fn := range fns {
		vTable.ComProcs[parentProcsCount+i] = windows.NewCallback(fn)
	}

	vTables[tName] = vTable
}

func typeInterfaceToString[T any]() (reflect.Type, string) {
	t := reflect.TypeOf((*T)(nil))
	if t.Kind() != reflect.Pointer {
		panic("must be a (*yourInterfaceType)(nil)")
	}
	t = t.Elem()
	return t, t.PkgPath() + "/" + t.Name()
}

func typeInterfaceToStringOnly[T any]() string {
	_, nane := typeInterfaceToString[T]()
	return nane
}

func guidOf[T any]() string {
	vtable := vTableOf[T]()
	if vtable == nil {
		return ""
	}
	return vtable.ComGUID
}

func vTableOf[T any]() *vTable {
	name := typeInterfaceToStringOnly[T]()
	vTablesL.Lock()
	defer vTablesL.Unlock()

	return vTables[name]
}

type ifceImpl interface {
	impl() any
	ifce() (*vTable, error)
}

type ifceDef[T any] struct {
	objImpl any
}

func (i ifceDef[T]) impl() any {
	return i.objImpl
}

func (i ifceDef[T]) ifce() (*vTable, error) {
	vtable := vTableOf[T]()
	if vtable == nil {
		return nil, fmt.Errorf("Unable to find vTable for %s", typeInterfaceToStringOnly[T]())
	}
	return vtable, nil
}
