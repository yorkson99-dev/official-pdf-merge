//go:build windows

package main

// Native COM IDispatch for Microsoft Office. Uses Windows DLLs and the Go
// standard library only: no PowerShell, command shells or external packages.
import (
    "errors"
    "fmt"
    "runtime"
    "syscall"
    "unicode/utf16"
    "unsafe"
)

type oleGUID struct {
    Data1 uint32
    Data2, Data3 uint16
    Data4 [8]byte
}
type oleVariant struct {
    Type, Reserved1, Reserved2, Reserved3 uint16
    Data [2]uint64
}
type oleDispatchParams struct {
    Args *oleVariant
    NamedArgs *int32
    Count, NamedCount uint32
}
type oleException struct {
    Code, Reserved uint16
    Source, Description, HelpFile uintptr
    HelpContext uint32
    ReservedPtr, DeferredFillIn uintptr
    Status int32
}
type oleDispatch struct { ptr uintptr }

var (
    iidIDispatch = oleGUID{Data1: 0x00020400, Data4: [8]byte{0xC0,0,0,0,0,0,0,0x46}}
    iidNull oleGUID
    ole32DLL = syscall.NewLazyDLL("ole32.dll")
    oleautDLL = syscall.NewLazyDLL("oleaut32.dll")
    coInitializeExProc = ole32DLL.NewProc("CoInitializeEx")
    coUninitializeProc = ole32DLL.NewProc("CoUninitialize")
    clsidFromProgIDProc = ole32DLL.NewProc("CLSIDFromProgID")
    coCreateInstanceProc = ole32DLL.NewProc("CoCreateInstance")
    sysAllocStringProc = oleautDLL.NewProc("SysAllocString")
    sysStringLenProc = oleautDLL.NewProc("SysStringLen")
    sysFreeStringProc = oleautDLL.NewProc("SysFreeString")
    variantClearProc = oleautDLL.NewProc("VariantClear")
)
const (
    coinitApartmentThreaded = 2
    clsctxInprocOrLocalServer = 5
    dispatchMethod = 1
    dispatchPropertyGet = 2
    dispatchPropertyPut = 4
    dispidPropertyPut int32 = -3
    vtEmpty = 0
    vtI4 = 3
    vtBSTR = 8
    vtDispatch = 9
    vtBool = 11
)
func oleFailed(hr uintptr) bool {return int32(uint32(hr))<0}
func oleHResult(hr uintptr) error {return fmt.Errorf("COM HRESULT 0x%08X",uint32(hr))}
func oleBSTR(s string) (oleVariant,error) {
    p,e:=syscall.UTF16PtrFromString(s);if e!=nil{return oleVariant{},e}
    b,_,_:=sysAllocStringProc.Call(uintptr(unsafe.Pointer(p)))
    if b==0{return oleVariant{},errors.New("無法配置 COM 字串")}
    return oleVariant{Type:vtBSTR,Data:[2]uint64{uint64(b)}},nil
}
func oleInteger(n int32) oleVariant {return oleVariant{Type:vtI4,Data:[2]uint64{uint64(uint32(n))}}}
func oleBoolean(b bool) oleVariant {
    if b{return oleVariant{Type:vtBool,Data:[2]uint64{0xffff}}}
    return oleVariant{Type:vtBool}
}
func oleClear(v *oleVariant) {if v!=nil{variantClearProc.Call(uintptr(unsafe.Pointer(v)))}}
func oleString(b uintptr) string {
    if b==0{return ""}
    n,_,_:=sysStringLenProc.Call(b);if n==0||n>32768{return ""}
    return string(utf16.Decode(unsafe.Slice((*uint16)(unsafe.Pointer(b)),int(n))))
}
func (e *oleException) message() string {
    desc:=oleString(e.Description)
    if e.Source!=0{sysFreeStringProc.Call(e.Source)}
    if e.Description!=0{sysFreeStringProc.Call(e.Description)}
    if e.HelpFile!=0{sysFreeStringProc.Call(e.HelpFile)}
    return desc
}
func oleMethod(obj uintptr,slot uintptr) uintptr {
    vt:=*(*uintptr)(unsafe.Pointer(obj))
    return *(*uintptr)(unsafe.Pointer(vt+slot*unsafe.Sizeof(uintptr(0))))
}
func (d *oleDispatch) release() {
    if d==nil||d.ptr==0{return}
    syscall.SyscallN(oleMethod(d.ptr,2),d.ptr)
    d.ptr=0
}
func oleStart(progID string) (*oleDispatch,error) {
    prog,e:=syscall.UTF16PtrFromString(progID);if e!=nil{return nil,e}
    var clsid oleGUID
    hr,_,_:=clsidFromProgIDProc.Call(uintptr(unsafe.Pointer(prog)),uintptr(unsafe.Pointer(&clsid)))
    if oleFailed(hr){return nil,fmt.Errorf("找不到 %s：%w",progID,oleHResult(hr))}
    var obj uintptr
    hr,_,_=coCreateInstanceProc.Call(uintptr(unsafe.Pointer(&clsid)),0,clsctxInprocOrLocalServer,
        uintptr(unsafe.Pointer(&iidIDispatch)),uintptr(unsafe.Pointer(&obj)))
    if oleFailed(hr)||obj==0{return nil,fmt.Errorf("無法啟動 %s：%w",progID,oleHResult(hr))}
    return &oleDispatch{ptr:obj},nil
}
func (d *oleDispatch) resolve(name string) (int32,error) {
    if d==nil||d.ptr==0{return 0,errors.New("COM 物件已關閉")}
    p,e:=syscall.UTF16PtrFromString(name);if e!=nil{return 0,e}
    ptr:=uintptr(unsafe.Pointer(p))
    var id int32
    hr,_,_:=syscall.SyscallN(oleMethod(d.ptr,5),d.ptr,uintptr(unsafe.Pointer(&iidNull)),
        uintptr(unsafe.Pointer(&ptr)),1,0,uintptr(unsafe.Pointer(&id)))
    runtime.KeepAlive(p)
    if oleFailed(hr){return 0,fmt.Errorf("Office 不支援 %s：%w",name,oleHResult(hr))}
    return id,nil
}
func (d *oleDispatch) invoke(name string,flag uint16,args ...oleVariant) (oleVariant,error) {
    var result oleVariant
    reverse:=make([]oleVariant,len(args))
    for i,arg:=range args{reverse[len(args)-i-1]=arg}
    defer func(){for i:=range reverse{oleClear(&reverse[i])}}()
    id,e:=d.resolve(name);if e!=nil{return result,e}
    var params oleDispatchParams
    if len(reverse)!=0{params.Args=&reverse[0];params.Count=uint32(len(reverse))}
    named:=dispidPropertyPut
    if flag==dispatchPropertyPut{params.NamedArgs=&named;params.NamedCount=1}
    var ex oleException
    var argErr uint32
    hr,_,_:=syscall.SyscallN(oleMethod(d.ptr,6),d.ptr,uintptr(uint32(id)),
        uintptr(unsafe.Pointer(&iidNull)),0,uintptr(flag),uintptr(unsafe.Pointer(&params)),
        uintptr(unsafe.Pointer(&result)),uintptr(unsafe.Pointer(&ex)),uintptr(unsafe.Pointer(&argErr)))
    runtime.KeepAlive(reverse)
    reason:=ex.message()
    if oleFailed(hr){
        oleClear(&result)
        if reason!=""{return oleVariant{},fmt.Errorf("Office %s：%s（%w）",name,reason,oleHResult(hr))}
        return oleVariant{},fmt.Errorf("Office %s：%w（參數 %d）",name,oleHResult(hr),argErr)
    }
    return result,nil
}
func (d *oleDispatch) put(name string,v oleVariant) error {
    result,e:=d.invoke(name,dispatchPropertyPut,v)
    if e==nil{oleClear(&result)}
    return e
}
func oleObjectFromVariant(v *oleVariant,name string) (*oleDispatch,error) {
    if v.Type!=vtDispatch||v.Data[0]==0{
        oleClear(v)
        return nil,fmt.Errorf("Office %s 未回傳物件",name)
    }
    obj:=&oleDispatch{ptr:uintptr(v.Data[0])}
    v.Type=vtEmpty;v.Data[0]=0
    oleClear(v)
    return obj,nil
}
func (d *oleDispatch) getObject(name string) (*oleDispatch,error) {
    v,e:=d.invoke(name,dispatchPropertyGet);if e!=nil{return nil,e}
    return oleObjectFromVariant(&v,name)
}
func (d *oleDispatch) objectMethod(name string,args ...oleVariant) (*oleDispatch,error) {
    v,e:=d.invoke(name,dispatchMethod,args...);if e!=nil{return nil,e}
    return oleObjectFromVariant(&v,name)
}
func (d *oleDispatch) method(name string,args ...oleVariant) error {
    v,e:=d.invoke(name,dispatchMethod,args...)
    if e==nil{oleClear(&v)}
    return e
}
func (d *oleDispatch) textMethod(name,text string,tail ...oleVariant) error {
    v,e:=oleBSTR(text);if e!=nil{return e}
    return d.method(name,append([]oleVariant{v},tail...)...)
}
func (d *oleDispatch) textObjectMethod(name,text string,tail ...oleVariant) (*oleDispatch,error) {
    v,e:=oleBSTR(text);if e!=nil{return nil,e}
    return d.objectMethod(name,append([]oleVariant{v},tail...)...)
}

func convertOfficeCOM(input,output,kind string) error {
    // Office COM objects and message calls must remain on one STA thread.
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    hr,_,_:=coInitializeExProc.Call(0,coinitApartmentThreaded)
    if oleFailed(hr){return fmt.Errorf("初始化 COM 失敗：%w",oleHResult(hr))}
    defer coUninitializeProc.Call()
    switch kind {
    case "word": return convertWordCOM(input,output)
    case "excel": return convertExcelCOM(input,output)
    case "powerpoint": return convertPowerPointCOM(input,output)
    }
    return fmt.Errorf("不支援的 Office 類型：%s",kind)
}
func convertWordCOM(input,output string) error {
    app,e:=oleStart("Word.Application");if e!=nil{return e}
    defer app.release();defer func(){_=app.method("Quit",oleInteger(0))}()
    if e=app.put("Visible",oleBoolean(false));e!=nil{return e}
    if e=app.put("DisplayAlerts",oleInteger(0));e!=nil{return e}
    if e=app.put("AutomationSecurity",oleInteger(3));e!=nil{return fmt.Errorf("未確認巨集已停用：%w",e)}
    docs,e:=app.getObject("Documents");if e!=nil{return e}
    defer docs.release()
    doc,e:=docs.textObjectMethod("Open",input,oleBoolean(false),oleBoolean(true),oleBoolean(false))
    if e!=nil{return e}
    defer doc.release();defer func(){_=doc.method("Close",oleInteger(0))}()
    if e=doc.textMethod("ExportAsFixedFormat",output,oleInteger(17));e!=nil{return e}
    return verifyConvertedPDF(output)
}
func convertExcelCOM(input,output string) error {
    app,e:=oleStart("Excel.Application");if e!=nil{return e}
    defer app.release();defer func(){_=app.method("Quit")}()
    for _,entry:=range []struct{name string;v oleVariant}{
        {"Visible",oleBoolean(false)},{"DisplayAlerts",oleBoolean(false)},
        {"AskToUpdateLinks",oleBoolean(false)},{"AutomationSecurity",oleInteger(3)},
    } {if e=app.put(entry.name,entry.v);e!=nil{return fmt.Errorf("Excel 設定 %s：%w",entry.name,e)}}
    books,e:=app.getObject("Workbooks");if e!=nil{return e}
    defer books.release()
    book,e:=books.textObjectMethod("Open",input,oleInteger(0),oleBoolean(true))
    if e!=nil{return e}
    defer book.release();defer func(){_=book.method("Close",oleBoolean(false))}()
    file,e:=oleBSTR(output);if e!=nil{return e}
    if e=book.method("ExportAsFixedFormat",oleInteger(0),file);e!=nil{return e}
    return verifyConvertedPDF(output)
}
func convertPowerPointCOM(input,output string) error {
    app,e:=oleStart("PowerPoint.Application");if e!=nil{return e}
    defer app.release();defer func(){_=app.method("Quit")}()
    if e=app.put("AutomationSecurity",oleInteger(3));e!=nil{return fmt.Errorf("未確認巨集已停用：%w",e)}
    ps,e:=app.getObject("Presentations");if e!=nil{return e}
    defer ps.release()
    presentation,e:=ps.textObjectMethod("Open",input,oleInteger(-1),oleInteger(0),oleInteger(0))
    if e!=nil{return e}
    defer presentation.release();defer func(){_=presentation.method("Close")}()
    if e=presentation.textMethod("SaveAs",output,oleInteger(32));e!=nil{return e}
    return verifyConvertedPDF(output)
}
