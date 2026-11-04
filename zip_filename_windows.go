//go:build windows
package main

// A traditional government-system ZIP may contain CP950 encoded names instead
// of UTF-8 names. This is display/sort conversion only; archive lookup uses the
// original entry index, not converted paths.
import (
 "syscall"
 "unicode/utf16"
 "unicode/utf8"
 "unsafe"
)
var zipMultiByteToWideChar=syscall.NewLazyDLL("kernel32.dll").NewProc("MultiByteToWideChar")
func displayZIPFileName(s string)string{
 if utf8.ValidString(s)||len(s)==0{return s}
 raw:=[]byte(s)
 n,_,_:=zipMultiByteToWideChar.Call(950,0,uintptr(unsafe.Pointer(&raw[0])),uintptr(len(raw)),0,0)
 if n==0||n>32768{return s}
 wide:=make([]uint16,n)
 actual,_,_:=zipMultiByteToWideChar.Call(950,0,uintptr(unsafe.Pointer(&raw[0])),uintptr(len(raw)),uintptr(unsafe.Pointer(&wide[0])),n)
 if actual!=n{return s}
 return string(utf16.Decode(wide))
}
