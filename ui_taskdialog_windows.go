//go:build windows

package main

import (
    "fmt"
    "syscall"
    "unsafe"
)

// Use the native Windows Task Dialog (Common Controls v6): a prominent,
// larger main instruction and two clearly labelled command-link actions.
// Keeping the dialog owned by the application's window makes it modal
// without introducing an external UI framework or sidecar files.
var taskDialogDLL=syscall.NewLazyDLL("comctl32.dll")
var taskDialogIndirectProc=taskDialogDLL.NewProc("TaskDialogIndirect")

const (
    taskDialogUseCommandLinks=0x00000010
    taskDialogAllowCancel=0x00000008
    taskDialogRelativeToWindow=0x00001000
    taskDialogSizeToContent=0x01000000
    taskDialogContinue=1001
    taskDialogStop=1002
)

type taskDialogButton struct {
    id int32
    text *uint16
}

type taskDialogConfig struct {
    size uint32
    hwndParent uintptr
    hInstance uintptr
    flags uint32
    commonButtons uint32
    windowTitle *uint16
    mainIcon uintptr
    instruction *uint16
    content *uint16
    buttonCount uint32
    buttons *taskDialogButton
    defaultButton int32
    radioCount uint32
    radios uintptr
    defaultRadio int32
    verification *uint16
    expandedInfo *uint16
    expandedControl *uint16
    collapsedControl *uint16
    footerIcon uintptr
    footerText *uint16
    callback uintptr
    callbackData uintptr
    width uint32
}

func confirmModernMixedMerge(count int) bool {
    // If Common Controls v6 cannot be loaded, fail closed: show an explicit
    // native caution with Stop selected; never bypass the user's warning.
    if err:=taskDialogIndirectProc.Find();err!=nil{
        traceStartup("TaskDialogIndirect unavailable: "+err.Error())
        return legacyMixedWarning(count)
    }
    buttons:=[2]taskDialogButton{
        {id:taskDialogStop,text:str("停止合併\n不處理本次文件")},
        {id:taskDialogContinue,text:str("仍要繼續\n已了解安全軟體可能攔截")},
    }
    cfg:=taskDialogConfig{
        hwndParent:hwndMain,
        flags:taskDialogUseCommandLinks|taskDialogAllowCancel|taskDialogRelativeToWindow,
        windowTitle:str("公文PDF快合｜慎用提醒"),
        mainIcon:0, // Use no task dialog icon while validating v6 native API
        instruction:str("混合格式文件較多，是否繼續？"),
        content:str(fmt.Sprintf("共%d份文件，包含非PDF附件。\nApex One可能攔截合併操作。",count)),
        buttonCount:uint32(len(buttons)),
        buttons:&buttons[0],
        defaultButton:taskDialogStop,
        width:0,
    }
    cfg.size=uint32(unsafe.Sizeof(cfg))
    var selected int32
    hr,_,_:=taskDialogIndirectProc.Call(uintptr(unsafe.Pointer(&cfg)),uintptr(unsafe.Pointer(&selected)),0,0)
    if int32(hr)<0 {
        traceStartup(fmt.Sprintf("TaskDialogIndirect HRESULT=0x%08X size=%d",uint32(hr),cfg.size))
        return legacyMixedWarning(count)
    }
    traceStartup(fmt.Sprintf("TaskDialogIndirect success button=%d",selected))
    return selected==taskDialogContinue
}

func legacyMixedWarning(count int) bool {
    // Only for systems where TaskDialogIndirect is unavailable.
    message:=fmt.Sprintf("混合格式共%d份，可能遭Apex One攔截。\n\n是否仍要繼續合併？",count)
    result,_,_:=pMessageBox.Call(hwndMain,
        uintptr(unsafe.Pointer(str(message))),
        uintptr(unsafe.Pointer(str("公文PDF快合｜慎用提醒"))),
        MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2)
    return result==IDYES
}
