package sqlbuilder

import (
	"reflect"
	"runtime"
	"sync"
	"unsafe"

	"github.com/huandu/go-clone"
)

const maxCloneRootCapacity = 32 << 10

type cloneScope struct {
	roots []unsafe.Pointer
	mu    sync.Mutex
}

var (
	cloneScopePool        = sync.Pool{New: func() any { return &cloneScope{} }}
	cloneAllocatorMethods = clone.AllocatorMethods{
		New: func(pool unsafe.Pointer, t reflect.Type) reflect.Value {
			value := reflect.New(t)
			scope := (*cloneScope)(pool)
			return scope.keep(value)
		},
		MakeSlice: func(pool unsafe.Pointer, t reflect.Type, length, capacity int) reflect.Value {
			value := reflect.MakeSlice(t, length, capacity)
			scope := (*cloneScope)(pool)
			return scope.keep(value)
		},
		MakeMap: func(pool unsafe.Pointer, t reflect.Type, size int) reflect.Value {
			value := reflect.MakeMapWithSize(t, size)
			scope := (*cloneScope)(pool)
			return scope.keep(value)
		},
		MakeChan: func(pool unsafe.Pointer, t reflect.Type, size int) reflect.Value {
			value := reflect.MakeChan(t, size)
			scope := (*cloneScope)(pool)
			return scope.keep(value)
		},
	}
)

func cloneWithGC(value any) any {
	scope := cloneScopePool.Get().(*cloneScope)
	defer func() {
		runtime.KeepAlive(value)
		scope.reset()
		cloneScopePool.Put(scope)
	}()

	allocator := clone.NewAllocator(unsafe.Pointer(scope), &cloneAllocatorMethods)
	return clone.MakeCloner(allocator).Clone(value)
}

func (scope *cloneScope) keep(value reflect.Value) reflect.Value {
	scope.mu.Lock()
	scope.roots = append(scope.roots, value.UnsafePointer())
	scope.mu.Unlock()
	return value
}

func (scope *cloneScope) reset() {
	clear(scope.roots)
	if cap(scope.roots) > maxCloneRootCapacity {
		scope.roots = nil
	}

	scope.roots = scope.roots[:0]
}
