//go:build windows

package main

import (
    "runtime"
    "testing"
    "unsafe"
)

// Windows always has COM infrastructure even on runners without Office.
// Scripting.Dictionary exercises exactly the native IDispatch vtable, VARIANT
// and BSTR paths that the Word/Excel/PowerPoint converters use.
func TestNativeCOMDispatchOnWindows(t *testing.T) {
    if unsafe.Sizeof(oleVariant{})!=24 {t.Fatalf("VARIANT size: %d",unsafe.Sizeof(oleVariant{}))}
    if unsafe.Sizeof(oleDispatchParams{})!=24 {t.Fatalf("DISPPARAMS size: %d",unsafe.Sizeof(oleDispatchParams{}))}
    if unsafe.Sizeof(oleException{})!=64 {t.Fatalf("EXCEPINFO size: %d",unsafe.Sizeof(oleException{}))}
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    hr,_,_:=coInitializeExProc.Call(0,coinitApartmentThreaded)
    if oleFailed(hr){t.Fatalf("CoInitializeEx: %v",oleHResult(hr))}
    defer coUninitializeProc.Call()
    d,e:=oleStart("Scripting.Dictionary")
    if e!=nil{t.Fatal(e)}
    defer d.release()
    if e=d.put("CompareMode",oleInteger(1));e!=nil{t.Fatal(e)}
    key,e:=oleBSTR("official-pdf-com-check")
    if e!=nil{t.Fatal(e)}
    if e=d.method("Add",key,oleInteger(37));e!=nil{t.Fatal(e)}
    count,e:=d.invoke("Count",dispatchPropertyGet)
    if e!=nil{t.Fatal(e)}
    if count.Type!=vtI4||int32(count.Data[0])!=1{t.Fatalf("COM Count: %+v",count)}
    oleClear(&count)
    lookup,e:=oleBSTR("official-pdf-com-check")
    if e!=nil{t.Fatal(e)}
    found,e:=d.invoke("Exists",dispatchMethod,lookup)
    if e!=nil{t.Fatal(e)}
    if found.Type!=vtBool||uint16(found.Data[0])!=0xffff{t.Fatalf("COM Exists: %+v",found)}
    oleClear(&found)
}
