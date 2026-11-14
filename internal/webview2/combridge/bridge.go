//go:build windows

package combridge

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	comIfcePointersL sync.RWMutex
	comIfcePointers  = map[uintptr]*comObject{}
)

func Resolve[T IUnknown](ifceP uintptr) T {
	comIfcePointersL.RLock()
	comObj := comIfcePointers[ifceP]
	comIfcePointersL.RUnlock()

	var n T
	if comObj != nil {
		t := comObj.resolve(ifceP)
		if t != nil {
			n = t.(T)
		}
	}

	return n
}

func New[T IUnknown](obj T) *ComObject[T] {
	cObj := new(
		ifceDef[T]{obj},
	)
	return newComObject[T](cObj)
}

func New2[T IUnknown, T2 IUnknown](obj T, obj2 T2) *ComObject[T] {
	cObj := new(
		ifceDef[T]{obj},
		ifceDef[T2]{obj2},
	)
	return newComObject[T](cObj)
}

func new(impls ...ifceImpl) *comObject {
	impls = append([]ifceImpl{ifceDef[IUnknown]{}}, impls...)

	cObj := &comObject{
		refCount:  1,
		ifces:     map[string]int{},
		ifcesImpl: make([]comInterfaceDesc, len(impls)),
	}

	for i, ifceDef := range impls {
		vtable, err := ifceDef.ifce()
		if err != nil {
			panic(err)
		}

		needsImplement := false
		for table := vtable; table != nil; table = table.Parent {
			guid := table.ComGUID
			if i, found := cObj.ifces[guid]; found {
				if guid == iUnknownGUID {
				} else if cObj.ifcesImpl[i].impl != ifceDef.impl() {
					panic(fmt.Sprintf("Interface '%s' is already implemented by another object", table.Name))
				}

				break
			}

			needsImplement = true
			cObj.ifces[guid] = i
		}

		if !needsImplement {
			continue
		}

		ifceP, ifcePSlice := allocUintptrObject(1)
		ifcePSlice[0] = vtable.ComVTable
		cObj.ifcesImpl[i] = comInterfaceDesc{ifceP, ifceDef.impl()}
	}

	comIfcePointersL.Lock()
	for _, ifceImpl := range cObj.ifcesImpl {
		comIfcePointers[ifceImpl.ref] = cObj
	}
	comIfcePointersL.Unlock()

	return cObj
}

func newComObject[T IUnknown](comObj *comObject) *ComObject[T] {
	c := &ComObject[T]{obj: comObj}
	runtime.SetFinalizer(c, func(obj *ComObject[T]) { obj.close(true) })
	return c
}

type ComObject[T IUnknown] struct {
	obj    *comObject
	closed int32
}

func (o *ComObject[T]) Ref() uintptr {
	if atomic.LoadInt32(&o.closed) != 0 {
		panic("ComObject has been released")
	}
	return o.obj.queryInterface(guidOf[T](), false)
}

func (o *ComObject[T]) Close() error {
	o.close(false)
	return nil
}

func (o *ComObject[T]) close(asyncRelease bool) {
	if atomic.CompareAndSwapInt32(&o.closed, 0, 1) {
		runtime.SetFinalizer(o, nil)
		if asyncRelease {
			go o.obj.release()
		} else {
			o.obj.release()
		}
	}
}

type comInterfaceDesc struct {
	ref  uintptr
	impl any
}

type comObject struct {
	l sync.Mutex

	refCount  int32
	ifces     map[string]int
	ifcesImpl []comInterfaceDesc
}

func (c *comObject) queryInterface(ifceGUID string, withAddRef bool) uintptr {
	c.l.Lock()
	defer c.l.Unlock()
	if c.refCount <= 0 {
		panic("call on released com object")
	}

	i, found := c.ifces[ifceGUID]
	if !found {
		return 0
	}

	if withAddRef {
		c.refCount++
	}
	return c.ifcesImpl[i].ref
}

func (c *comObject) resolve(ifceP uintptr) any {
	c.l.Lock()
	defer c.l.Unlock()
	if c.refCount <= 0 {
		panic("call on destroyed com object")
	}

	for _, ifce := range c.ifcesImpl {
		if ifce.ref != ifceP {
			continue
		}

		return ifce.impl
	}
	return nil
}

func (c *comObject) addRef() int32 {
	c.l.Lock()
	defer c.l.Unlock()
	if c.refCount <= 0 {
		panic("call on destroyed com object")
	}

	c.refCount++
	return c.refCount
}

func (c *comObject) release() int32 {
	c.l.Lock()
	defer c.l.Unlock()
	if c.refCount <= 0 {
		panic("call on destroyed com object")
	}

	if c.refCount--; c.refCount == 0 {
		comIfcePointersL.Lock()
		for _, ref := range c.ifcesImpl {
			delete(comIfcePointers, ref.ref)
		}
		comIfcePointersL.Unlock()

		for _, impl := range c.ifcesImpl {
			ref := impl.ref
			if ref == 0 {
				continue
			}

			globalFree(ref)
		}
	}

	return c.refCount
}
