//go:build windows

package desktop

import (
	"syscall"
	"unsafe"
)

// SystemLocale 读取 Windows 用户界面语言，失败时交由调用方回退到英语。
func SystemLocale() string {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	language, _, _ := kernel.NewProc("GetUserDefaultUILanguage").Call()
	if language == 0 {
		return ""
	}
	var name [85]uint16
	n, _, _ := kernel.NewProc("LCIDToLocaleName").Call(language, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)), 0)
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(name[:])
}
