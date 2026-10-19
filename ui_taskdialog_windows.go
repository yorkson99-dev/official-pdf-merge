//go:build windows

package main

import (
    "fmt"
    "syscall"
    "unsafe"
)

// Modern, compact native Win32 confirmation card. No HTML runtime, sidecar
// settings file, or dependency on TaskDialogIndirect availability. The stop
// action is always focused first and closing the window also means stop.
const (
    taskDialogContinue=1001
    taskDialogStop=1002
    warningClassName="FastPDFRiskDialog"
)
var (
    warningDialogProc=syscall.NewCallback(warningWndProc)
    warningClassRegistered bool
    warningDialogHWND uintptr
    warningDialogDone bool
    warningDialogAccepted bool
    warningButtonStop uintptr
    pIsDialogMessage=u32.NewProc("IsDialogMessageW")
    pGetWindowRect=u32.NewProc("GetWindowRect")
)

func riskChild(parent uintptr,class,text string,x,y,w,h int,style uint32,id int) uintptr{
    instance,_,_:=pGetModuleHandle.Call(0)
    handle,_,_:=pCreateWindowEx.Call(0,
        uintptr(unsafe.Pointer(str(class))),uintptr(unsafe.Pointer(str(text))),
        uintptr(WS_CHILD|WS_VISIBLE|style),
        uintptr(x),uintptr(y),uintptr(w),uintptr(h),
        parent,uintptr(id),instance,0)
    if handle!=0&&fontHandle!=0{send(handle,WM_SETFONT,fontHandle,1)}
    return handle
}

func paintRiskButton(l uintptr){
    d:=(*drawItem)(unsafe.Pointer(l))
    if d==nil{return}
    primary:=d.CtlID==taskDialogStop
    bg,fg,border:=winColor(244,248,254),winColor(35,77,133),winColor(213,226,245)
    label:="仍要繼續"
    if primary {
        bg=winColor(36,104,206);fg=winColor(255,255,255);border=bg
        label="停止合併"
    }
    if d.State&ODS_SELECTED!=0 {
        if primary{bg=winColor(21,82,171)}else{bg=winColor(225,238,255)}
    }
    roundedColor(d.Hdc,d.RcItem,16,bg,border)
    pSetBkMode.Call(d.Hdc,1)
    pSetTextColor.Call(d.Hdc,fg)
    pDrawText.Call(d.Hdc,uintptr(unsafe.Pointer(str(label))),^uintptr(0),
        uintptr(unsafe.Pointer(&d.RcItem)),DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func warningWndProc(h uintptr,m uint32,w,l uintptr)uintptr{
    switch m {
    case WM_CREATE:
        // The visual hierarchy uses a large headline, two short explanatory
        // sentences and two large buttons rather than a legacy MessageBox.
        icon:=riskChild(h,"STATIC","!",35,31,40,36,SS_LEFTNOWORDWRAP,0)
        if titleFont!=0{send(icon,WM_SETFONT,titleFont,1)}
        title:=riskChild(h,"STATIC","混合格式合併提醒",81,32,420,44,SS_LEFTNOWORDWRAP,0)
        if titleFont!=0{send(title,WM_SETFONT,titleFont,1)}
        riskChild(h,"STATIC",fmt.Sprintf("本次合併共%d份文件，包含非PDF附件。",riskWarningCount),
            43,114,477,29,SS_LEFTNOWORDWRAP,0)
        riskChild(h,"STATIC","Apex One可能攔截此操作，請審慎確認。",
            43,150,480,33,SS_LEFTNOWORDWRAP,0)
        warningButtonStop=riskChild(h,"BUTTON","停止合併",43,212,237,45,
            WS_TABSTOP|BS_OWNERDRAW,taskDialogStop)
        riskChild(h,"BUTTON","仍要繼續",296,212,237,45,
            WS_TABSTOP|BS_OWNERDRAW,taskDialogContinue)
        pSetFocus.Call(warningButtonStop)
        return 0
    case WM_ERASEBKGND:
        return 1
    case WM_PAINT:
        var ps paintStruct
        dc,_,_:=pBeginPaint.Call(h,uintptr(unsafe.Pointer(&ps)))
        if dc!=0 {
            fillColor(dc,winRect{0,0,572,284},winColor(255,255,255))
            roundedColor(dc,winRect{31,30,73,74},18,
                winColor(255,247,224),winColor(255,225,154))
            fillColor(dc,winRect{42,198,535,199},winColor(230,235,243))
            pEndPaint.Call(h,uintptr(unsafe.Pointer(&ps)))
        }
        return 0
    case WM_CTLCOLORSTATIC:
        pSetBkMode.Call(w,1)
        pSetTextColor.Call(w,winColor(34,57,89))
        brush,_,_:=pGetStockObject.Call(5)
        return brush
    case WM_DRAWITEM:
        paintRiskButton(l)
        return 1
    case WM_COMMAND:
        id:=int(w&0xFFFF)
        if id==taskDialogStop||id==taskDialogContinue{
            warningDialogAccepted=id==taskDialogContinue
            pDestroyWindow.Call(h)
            return 0
        }
    case WM_KEYDOWN:
        if w==0x1B { // Escape means stop
            pDestroyWindow.Call(h)
            return 0
        }
    case 0x0010: // WM_CLOSE; clicking X also cancels
        pDestroyWindow.Call(h)
        return 0
    case WM_DESTROY:
        warningDialogDone=true
        warningDialogHWND=0
        return 0
    }
    result,_,_:=pDefWindowProc.Call(h,uintptr(m),w,l)
    return result
}

var riskWarningCount int

func confirmModernMixedMerge(count int)bool {
    riskWarningCount=count
    warningDialogDone=false
    warningDialogAccepted=false
    instance,_,_:=pGetModuleHandle.Call(0)
    className:=str(warningClassName)
    if !warningClassRegistered{
        wc:=wndClass{
            cbSize:uint32(unsafe.Sizeof(wndClass{})),
            lpfnWndProc:warningDialogProc,
            hInstance:instance,hbrBackground:6,
            lpszClassName:className,
        }
        atom,_,_:=pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
        if atom==0{return legacyMixedWarning(count)}
        warningClassRegistered=true
    }
    var parent winRect
    pGetWindowRect.Call(hwndMain,uintptr(unsafe.Pointer(&parent)))
    x:=int(parent.Left)+(int(parent.Right-parent.Left)-572)/2
    y:=int(parent.Top)+(int(parent.Bottom-parent.Top)-320)/2
    pEnableWindow.Call(hwndMain,0)
    hwnd,_,_:=pCreateWindowEx.Call(0,
        uintptr(unsafe.Pointer(className)),
        uintptr(unsafe.Pointer(str("公文PDF快合｜慎用提醒"))),
        uintptr(WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_VISIBLE),
        uintptr(x),uintptr(y),572,320,
        hwndMain,0,instance,0)
    if hwnd==0{
        pEnableWindow.Call(hwndMain,1)
        return legacyMixedWarning(count)
    }
    warningDialogHWND=hwnd
    if warningButtonStop!=0{pSetFocus.Call(warningButtonStop)}
    var m msg
    for !warningDialogDone {
        r,_,_:=pGetMessage.Call(uintptr(unsafe.Pointer(&m)),0,0,0)
        if int32(r)<=0{break}
        handled,_,_:=pIsDialogMessage.Call(hwnd,uintptr(unsafe.Pointer(&m)))
        if handled!=0{continue}
        pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
        pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
    }
    if !warningDialogDone {pDestroyWindow.Call(hwnd)}
    pEnableWindow.Call(hwndMain,1)
    pSetFocus.Call(hwndMain)
    return warningDialogAccepted
}

func legacyMixedWarning(count int)bool{
    text:=fmt.Sprintf("混合格式共%d份，Apex One可能攔截。\n\n是否繼續合併？",count)
    selected,_,_:=pMessageBox.Call(hwndMain,
        uintptr(unsafe.Pointer(str(text))),
        uintptr(unsafe.Pointer(str("公文PDF快合｜慎用提醒"))),
        MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2)
    return selected==IDYES
}
