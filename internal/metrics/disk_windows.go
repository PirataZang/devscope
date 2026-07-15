//go:build windows

package metrics

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32          = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpace = modkernel32.NewProc("GetDiskFreeSpaceExW")
)

func getDiskSpace(path string) (total, free uint64, err error) {
	p := path
	if p == "/" {
		p = "C:\\"
	}
	pathPtr, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return 0, 0, err
	}
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	r1, _, e1 := procGetDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if r1 == 0 {
		return 0, 0, e1
	}
	return totalNumberOfBytes, freeBytesAvailable, nil
}
