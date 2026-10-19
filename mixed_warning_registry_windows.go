//go:build windows

package main

import (
    "syscall"
    "unsafe"
)

// HKCU is per Windows account. No .ini, local settings file or network call.
// Moving the single portable EXE to another PC keeps default warning=enabled.
const mixedWarningRegistrySubkey = `Software\OfficialPDFMerge`
const mixedWarningRegistryValue = "ShowMixedFormatWarning"

const (
    hkeyCurrentUser = uintptr(0x80000001)
    regDWord = uintptr(4)
    keySetValue = uintptr(0x0002)
    rrfRegDWord = uintptr(0x00000010)
)
var (
    registryDLL = syscall.NewLazyDLL("advapi32.dll")
    regGetValue = registryDLL.NewProc("RegGetValueW")
    regCreateKey = registryDLL.NewProc("RegCreateKeyExW")
    regSetValue = registryDLL.NewProc("RegSetValueExW")
    regCloseKey = registryDLL.NewProc("RegCloseKey")
)

func readMixedWarningSetting() bool {
    var data uint32
    size := uint32(unsafe.Sizeof(data))
    key, err1 := syscall.UTF16PtrFromString(mixedWarningRegistrySubkey)
    value, err2 := syscall.UTF16PtrFromString(mixedWarningRegistryValue)
    if err1 != nil || err2 != nil { return true }
    rc, _, _ := regGetValue.Call(hkeyCurrentUser,
        uintptr(unsafe.Pointer(key)),uintptr(unsafe.Pointer(value)),
        rrfRegDWord, 0, uintptr(unsafe.Pointer(&data)),
        uintptr(unsafe.Pointer(&size)))
    if rc != 0 || size != 4 { return true }
    return data != 0
}

func writeMixedWarningSetting(enabled bool) error {
    key, err := syscall.UTF16PtrFromString(mixedWarningRegistrySubkey)
    if err != nil {return err}
    name, err := syscall.UTF16PtrFromString(mixedWarningRegistryValue)
    if err != nil {return err}
    var handle uintptr
    rc, _, _ := regCreateKey.Call(hkeyCurrentUser,
        uintptr(unsafe.Pointer(key)), 0, 0, 0, keySetValue, 0,
        uintptr(unsafe.Pointer(&handle)), 0)
    if rc != 0 {return syscall.Errno(rc)}
    defer regCloseKey.Call(handle)
    data := uint32(0)
    if enabled { data = 1 }
    rc, _, _ = regSetValue.Call(handle,uintptr(unsafe.Pointer(name)),0,
        regDWord,uintptr(unsafe.Pointer(&data)),uintptr(unsafe.Sizeof(data)))
    if rc != 0 {return syscall.Errno(rc)}
    return nil
}
