//go:build windows
package main

import (
 "fmt"
 "syscall"
 "unsafe"
)

const (
 zipChoiceClass = "FastPDFZipModeChoice"
 zipChoiceCancel = 3210
 zipChoiceLetter = 3211
 zipChoiceSign = 3212
 zipChoiceDraft = 3213
)

var (
 zipChoiceProc = syscall.NewCallback(zipChoiceWndProc)
 zipChoiceRegistered bool
 zipChoiceDone bool
 zipChoiceResult zipMergeMode
 zipChoiceAvailable []zipMergeMode
 zipChoiceFirstButton uintptr
)

func zipChoiceButtonID(mode zipMergeMode)int{
 switch mode{
 case zipModeLetter:return zipChoiceLetter
 case zipModeSign:return zipChoiceSign
 case zipModeDraft:return zipChoiceDraft
 }
 return 0
}
func zipChoiceModeID(id int)zipMergeMode{
 switch id{
 case zipChoiceLetter:return zipModeLetter
 case zipChoiceSign:return zipModeSign
 case zipChoiceDraft:return zipModeDraft
 }
 return ""
}
func zipChoiceButtonLabel(mode zipMergeMode)string{
 switch mode{
 case zipModeLetter:return "合併來文"
 case zipModeSign:return "合併簽"
 case zipModeDraft:return "合併稿"
 }
 return ""
}
func zipChoiceOutputLabel(mode zipMergeMode)string{
 switch mode{
 case zipModeLetter:return "原有來文命名規則"
 case zipModeSign:return "輸出：奉核簽.pdf"
 case zipModeDraft:return "輸出：奉核稿.pdf"
 }
 return ""
}

func drawZipChoiceButton(l uintptr){
 item:=(*drawItem)(unsafe.Pointer(l))
 if item==nil{return}
 mode:=zipChoiceModeID(int(item.CtlID))
 selected:=mode!=""
 bg,fg,border:=winColor(243,247,253),winColor(39,79,129),winColor(213,225,242)
 if selected{bg=winColor(229,241,255);fg=winColor(28,89,166)}
 if item.State&ODS_SELECTED!=0{bg=winColor(206,226,250)}
 if !selected{bg=winColor(248,249,252);fg=winColor(91,103,123)}
 roundedColor(item.Hdc,item.RcItem,14,bg,border)
 pSetBkMode.Call(item.Hdc,1)
 pSetTextColor.Call(item.Hdc,fg)
 label:=zipChoiceButtonLabel(mode)
 if !selected{label="取消"}
 pDrawText.Call(item.Hdc,uintptr(unsafe.Pointer(str(label))),^uintptr(0),
  uintptr(unsafe.Pointer(&item.RcItem)),DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func zipChoiceWndProc(h uintptr,m uint32,w,l uintptr)uintptr{
 switch m{
 case WM_CREATE:
  title:=riskChild(h,"STATIC","選擇本次公文合併類型",34,25,492,43,SS_LEFTNOWORDWRAP,0)
  if titleFont!=0{send(title,WM_SETFONT,titleFont,1)}
  riskChild(h,"STATIC","僅處理選擇的資料夾；不會混入另一類公文或參考附件。",35,73,492,26,SS_LEFTNOWORDWRAP,0)
  zipChoiceFirstButton=0
  for i,mode:=range zipChoiceAvailable{
   y:=110+i*66
   btn:=riskChild(h,"BUTTON",zipChoiceButtonLabel(mode),36,y,486,37,
    WS_TABSTOP|BS_OWNERDRAW,zipChoiceButtonID(mode))
   if zipChoiceFirstButton==0{zipChoiceFirstButton=btn}
   riskChild(h,"STATIC",zipChoiceOutputLabel(mode),51,y+37,455,23,SS_LEFTNOWORDWRAP,0)
  }
  riskChild(h,"BUTTON","取消",36,320,486,42,WS_TABSTOP|BS_OWNERDRAW,zipChoiceCancel)
  if zipChoiceFirstButton!=0{pSetFocus.Call(zipChoiceFirstButton)}
  return 0
 case WM_ERASEBKGND:return 1
 case WM_PAINT:
  var ps paintStruct
  dc,_,_:=pBeginPaint.Call(h,uintptr(unsafe.Pointer(&ps)))
  if dc!=0{
   fillColor(dc,winRect{0,0,560,400},winColor(255,255,255))
   pEndPaint.Call(h,uintptr(unsafe.Pointer(&ps)))
  }
  return 0
 case WM_CTLCOLORSTATIC:
  pSetBkMode.Call(w,1)
  pSetTextColor.Call(w,winColor(44,65,91))
  brush,_,_:=pGetStockObject.Call(5)
  return brush
 case WM_DRAWITEM:
  drawZipChoiceButton(l)
  return 1
 case WM_COMMAND:
  id:=int(w&0xFFFF)
  if id==zipChoiceCancel{
   pDestroyWindow.Call(h)
   return 0
  }
  selected:=zipChoiceModeID(id)
  if selected!=""{
   for _,allowed:=range zipChoiceAvailable{
    if allowed==selected{
     zipChoiceResult=selected
     pDestroyWindow.Call(h)
     return 0
    }
   }
  }
 case WM_KEYDOWN:
  if w==0x1B{pDestroyWindow.Call(h);return 0}
 case 0x0010:
  pDestroyWindow.Call(h)
  return 0
 case WM_DESTROY:
  zipChoiceDone=true
  return 0
 }
 result,_,_:=pDefWindowProc.Call(h,uintptr(m),w,l)
 return result
}

func chooseZIPMergeMode(modes []zipMergeMode)zipMergeMode{
 if len(modes)==0{return ""}
 zipChoiceAvailable=append([]zipMergeMode(nil),modes...)
 zipChoiceResult=""
 zipChoiceDone=false
 instance,_,_:=pGetModuleHandle.Call(0)
 class:=str(zipChoiceClass)
 if !zipChoiceRegistered{
  wc:=wndClass{cbSize:uint32(unsafe.Sizeof(wndClass{})),
   lpfnWndProc:zipChoiceProc,hInstance:instance,hbrBackground:6,lpszClassName:class}
  atom,_,_:=pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
  if atom==0{
   alert("無法開啟合併類型選擇視窗，已停止本次合併。",true)
   return ""
  }
  zipChoiceRegistered=true
 }
 var parent winRect
 pGetWindowRect.Call(hwndMain,uintptr(unsafe.Pointer(&parent)))
 x:=int(parent.Left)+(int(parent.Right-parent.Left)-560)/2
 y:=int(parent.Top)+(int(parent.Bottom-parent.Top)-410)/2
 pEnableWindow.Call(hwndMain,0)
 hwnd,_,_:=pCreateWindowEx.Call(0,
  uintptr(unsafe.Pointer(class)),uintptr(unsafe.Pointer(str("公文 PDF 快合｜選擇合併類型"))),
  uintptr(WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_VISIBLE),
  uintptr(x),uintptr(y),560,410,hwndMain,0,instance,0)
 if hwnd==0{
  pEnableWindow.Call(hwndMain,1)
  return ""
 }
 if zipChoiceFirstButton!=0{pSetFocus.Call(zipChoiceFirstButton)}
 var msg msg
 for !zipChoiceDone{
  r,_,_:=pGetMessage.Call(uintptr(unsafe.Pointer(&msg)),0,0,0)
  if int32(r)<=0{break}
  handled,_,_:=pIsDialogMessage.Call(hwnd,uintptr(unsafe.Pointer(&msg)))
  if handled!=0{continue}
  pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
  pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
 }
 if !zipChoiceDone{pDestroyWindow.Call(hwnd)}
 pEnableWindow.Call(hwndMain,1)
 pSetFocus.Call(hwndMain)
 return zipChoiceResult
}

func selectedZIPDescription(mode zipMergeMode, count int)string{
 if mode==zipModeLetter{return fmt.Sprintf("來文：%d 份文件",count)}
 return fmt.Sprintf("%s：%d 份文件（含主文 (after) PDF）",mode,count)
}
