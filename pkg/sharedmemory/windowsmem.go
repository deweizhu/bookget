package sharedmemory

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

var (
	modkernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procOpenFileMappingW = modkernel32.NewProc("OpenFileMappingW")
)

func openFileMapping(desiredAccess uint32, inheritHandle bool, name *uint16) (handle windows.Handle, err error) {
	inherit := 0
	if inheritHandle {
		inherit = 1
	}
	r1, _, e1 := procOpenFileMappingW.Call(
		uintptr(desiredAccess),
		uintptr(inherit),
		uintptr(unsafe.Pointer(name)),
	)
	if r1 == 0 {
		err = error(e1)
	} else {
		handle = windows.Handle(r1)
	}
	return
}
