//go:build windows

package disk

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func FreeBytes(path string) (uint64, error) {
	clean := filepath.Clean(path)
	if _, err := os.Stat(clean); err != nil {
		return 0, err
	}
	ptr, err := syscall.UTF16PtrFromString(clean)
	if err != nil {
		return 0, err
	}

	var freeAvailable uint64
	var totalBytes uint64
	var totalFree uint64
	ret, _, callErr := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(&freeAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if ret == 0 {
		return 0, callErr
	}
	return freeAvailable, nil
}
