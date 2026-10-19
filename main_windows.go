//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

// Native Win32 controls: fast startup, no .NET, Python, or web runtime.
var (
	u32                       = syscall.NewLazyDLL("user32.dll")
	g32                       = syscall.NewLazyDLL("gdi32.dll")
	k32                       = syscall.NewLazyDLL("kernel32.dll")
	s32                       = syscall.NewLazyDLL("shell32.dll")
	comdlg                    = syscall.NewLazyDLL("comdlg32.dll")
    cc32= syscall.NewLazyDLL("comctl32.dll")
    pInitCommonControlsEx=cc32.NewProc("InitCommonControlsEx")
	pRegisterClassEx          = u32.NewProc("RegisterClassExW")
	pCreateWindowEx           = u32.NewProc("CreateWindowExW")
	pDefWindowProc            = u32.NewProc("DefWindowProcW")
	pGetMessage               = u32.NewProc("GetMessageW")
	pTranslateMessage         = u32.NewProc("TranslateMessage")
	pDispatchMessage          = u32.NewProc("DispatchMessageW")
	pSendMessage              = u32.NewProc("SendMessageW")
	pPostMessage              = u32.NewProc("PostMessageW")
	pMessageBox               = u32.NewProc("MessageBoxW")
	pSetWindowText            = u32.NewProc("SetWindowTextW")
    pLoadIcon                 = u32.NewProc("LoadIconW")
    pBeginPaint               = u32.NewProc("BeginPaint")
    pEndPaint                 = u32.NewProc("EndPaint")
    pFillRect                 = u32.NewProc("FillRect")
    pDrawText                 = u32.NewProc("DrawTextW")
	pEnableWindow             = u32.NewProc("EnableWindow")
	pDestroyWindow            = u32.NewProc("DestroyWindow")
	pPostQuitMessage          = u32.NewProc("PostQuitMessage")
	pSetWindowLongPtr         = u32.NewProc("SetWindowLongPtrW")
	pCallWindowProc           = u32.NewProc("CallWindowProcW")
	pGetSystemMetrics         = u32.NewProc("GetSystemMetrics")
    pSystemParametersInfo=u32.NewProc("SystemParametersInfoW")
	pGetModuleHandle          = k32.NewProc("GetModuleHandleW")
	pDragAcceptFiles          = s32.NewProc("DragAcceptFiles")
	pDragQueryFile            = s32.NewProc("DragQueryFileW")
	pDragFinish               = s32.NewProc("DragFinish")
	pGetOpenFileName          = comdlg.NewProc("GetOpenFileNameW")
	pGetSaveFileName          = comdlg.NewProc("GetSaveFileNameW")
	pCreateFont               = g32.NewProc("CreateFontW")
    pCreateSolidBrush         = g32.NewProc("CreateSolidBrush")
    pCreatePen                = g32.NewProc("CreatePen")
    pSelectObject             = g32.NewProc("SelectObject")
    pDeleteObject             = g32.NewProc("DeleteObject")
    pRoundRect                = g32.NewProc("RoundRect")
    pSetBkMode                = g32.NewProc("SetBkMode")
    pSetTextColor             = g32.NewProc("SetTextColor")
    pSetBkColor               = g32.NewProc("SetBkColor")
    pGetStockObject           = g32.NewProc("GetStockObject")
	pSetProcessDPIAware       = u32.NewProc("SetProcessDPIAware")
	pGetKeyState              = u32.NewProc("GetKeyState")
	pSetFocus                 = u32.NewProc("SetFocus")
)

const (
	WS_OVERLAPPED        = 0x00000000
	WS_CAPTION           = 0x00C00000
	WS_SYSMENU           = 0x00080000
	WS_MINIMIZEBOX       = 0x00020000
	WS_CHILD             = 0x40000000
	WS_VISIBLE           = 0x10000000
	WS_BORDER            = 0x00800000
	WS_VSCROLL           = 0x00200000
	WS_TABSTOP           = 0x00010000
	WS_EX_CLIENTEDGE     = 0x00000200
	LBS_OWNERDRAWFIXED=0x0010
    LBS_HASSTRINGS=0x0040
    LBS_NOTIFY           = 0x0001
	LBS_NOINTEGRALHEIGHT = 0x0100
	WM_CREATE            = 0x0001
    WM_PAINT             = 0x000F
    WM_ERASEBKGND        = 0x0014
    WM_DRAWITEM          = 0x002B
    WM_MEASUREITEM=0x002C
    WM_CTLCOLORSTATIC    = 0x0138
    BS_OWNERDRAW         = 0x0000000B
    ODS_SELECTED         = 0x0001
    ODS_DISABLED         = 0x0004
    DT_CENTER            = 0x0001
    DT_VCENTER           = 0x0004
    DT_SINGLELINE        = 0x0020
    DT_END_ELLIPSIS=0x00008000
    ODT_LISTBOX=0x0002
    ICC_PROGRESS_CLASS=0x20
    PBM_SETPOS=0x0402
    PBM_SETRANGE32=0x0406
    SS_LEFTNOWORDWRAP    = 0x000C
    SS_NOPREFIX          = 0x0080
    SS_ENDELLIPSIS       = 0x4000
	WM_TIMER             = 0x0113
    checkpointExitTimer = 22001
    WM_DESTROY           = 0x0002
	WM_COMMAND           = 0x0111
	WM_DROPFILES         = 0x0233
	WM_SETFONT           = 0x0030
	WM_LBUTTONDOWN       = 0x0201
	WM_LBUTTONUP         = 0x0202
	WM_KEYDOWN           = 0x0100
	WM_APP               = 0x8000
	BN_CLICKED           = 0
	LB_ADDSTRING         = 0x0180
	LB_RESETCONTENT      = 0x0184
	LB_SETCURSEL         = 0x0186
	LB_GETCURSEL         = 0x0188
	LB_ITEMFROMPOINT     = 0x01A9
	MB_OK                = 0x0
	MB_ICONERROR         = 0x10
	MB_ICONINFORMATION   = 0x40
	MB_YESNO             = 0x4
    MB_ICONWARNING       = 0x30
    MB_DEFBUTTON2        = 0x100
	IDYES                = 6
    BM_SETCHECK          = 0x00F1
	BM_GETCHECK          = 0x00F0
	OFN_EXPLORER         = 0x00080000
	OFN_ALLOWMULTISELECT = 0x00000200
	OFN_FILEMUSTEXIST    = 0x00001000
	OFN_PATHMUSTEXIST    = 0x00000800
	OFN_NOCHANGEDIR      = 0x00000008
	GWLP_WNDPROC         = ^uintptr(3)
	VK_DELETE            = 0x2E
	VK_CONTROL           = 0x11
	VK_UP                = 0x26
	VK_DOWN              = 0x28
	ID_ADD               = 101
	ID_UP                = 104
	ID_DOWN              = 105
	ID_REMOVE            = 106
	ID_CLEAR             = 107
	ID_MERGE             = 108
	ID_ADD_ZIP           = 109
	ID_FULL_NAME         = 110
    ID_SAVE_AS           = 111
    ID_MIXED_WARNING     = 112
    ID_CHECKPOINT_ZIP = 113
	ID_LIST              = 201
	ID_STATUS            = 202
    ID_PROGRESS=203
    ID_COLUMN_ORDER=204
    ID_COLUMN_FILENAME=205
    ID_PROGRESS_LABEL=206
)

type wndClass struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}
type msg struct {
	hwnd     uintptr
	message  uint32
	_        uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	ptX      int32
	ptY      int32
	lPrivate uint32
}
type openFileName struct {
	lStructSize       uint32
	pad0              uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}
type winRect struct { Left, Top, Right, Bottom int32 }
type paintStruct struct {
    Hdc uintptr
    Erase int32
    PaintRect winRect
    Restore int32
    IncUpdate int32
    Reserved [32]byte
}
type measureItem struct {
 ctlType uint32
 ctlID uint32
 itemID uint32
 itemWidth uint32
 itemHeight uint32
 itemData uintptr
}
type drawItem struct {
    CtlType uint32
    CtlID uint32
    ItemID uint32
    Action uint32
    State uint32
    HwndItem uintptr
    Hdc uintptr
    RcItem winRect
    ItemData uintptr
}
func winColor(r,g,b byte) uintptr {
    return uintptr(uint32(r)|uint32(g)<<8|uint32(b)<<16)
}
func fillColor(hdc uintptr, rc winRect, rgb uintptr) {
    brush,_,_ := pCreateSolidBrush.Call(rgb)
    pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
    pDeleteObject.Call(brush)
}
func roundedColor(hdc uintptr, rc winRect, radius int32, fill, border uintptr) {
    brush,_,_ := pCreateSolidBrush.Call(fill)
    pen,_,_ := pCreatePen.Call(0,1,border)
    oldBrush,_,_ := pSelectObject.Call(hdc,brush)
    oldPen,_,_ := pSelectObject.Call(hdc,pen)
    pRoundRect.Call(hdc,uintptr(rc.Left),uintptr(rc.Top),uintptr(rc.Right),uintptr(rc.Bottom),uintptr(radius),uintptr(radius))
    pSelectObject.Call(hdc,oldPen)
    pSelectObject.Call(hdc,oldBrush)
    pDeleteObject.Call(pen)
    pDeleteObject.Call(brush)
}
func paintBackground(hwnd uintptr) {
    var ps paintStruct
    hdc,_,_:=pBeginPaint.Call(hwnd,uintptr(unsafe.Pointer(&ps)))
    if hdc==0 {return}
    bg:=winColor(247,249,253)
    fillColor(hdc,winRect{0,0,690,694},bg)
    roundedColor(hdc,winRect{22,106,668,305},24,winColor(233,243,255),winColor(217,232,253))
    roundedColor(hdc,winRect{22,321,668,577},24,winColor(255,255,255),winColor(229,235,243))
    roundedColor(hdc,winRect{22,581,668,620},13,winColor(255,255,255),winColor(229,235,243))
    roundedColor(hdc,winRect{22,628,668,685},13,winColor(236,242,249),winColor(236,242,249))
    pEndPaint.Call(hwnd,uintptr(unsafe.Pointer(&ps)))
}
func paintButton(l uintptr) {
    d:=(*drawItem)(unsafe.Pointer(l))
    if d==nil {return}
    var title string
    fg:=winColor(32,49,78)
    fill:=winColor(246,249,254)
    border:=winColor(222,232,245)
    switch d.CtlID {
    case ID_ADD_ZIP:
        title="  選擇公文 ZIP　›"
        fill=winColor(37,112,237);border=fill;fg=winColor(255,255,255)
    case ID_CHECKPOINT_ZIP:
        title="分段接續"
        fill=winColor(231,241,255);border=winColor(210,228,252);fg=winColor(24,101,224)
    case ID_MERGE:
        title="合併一般文件"
        fill=winColor(231,241,255);border=winColor(210,228,252);fg=winColor(24,101,224)
    case ID_ADD: title="＋ 加入文件"
    case ID_UP: title="↑ 上移"
    case ID_DOWN: title="↓ 下移"
    case ID_REMOVE: title="移除"
    case ID_CLEAR: title="清空"
    default: return
    }
    if d.State&ODS_SELECTED!=0 {fill=winColor(195,219,250);if d.CtlID==ID_ADD_ZIP {fill=winColor(24,89,198)}}
    if d.State&ODS_DISABLED!=0 {fill=winColor(235,238,243);fg=winColor(152,158,168)}
    roundedColor(d.Hdc,d.RcItem,16,fill,border)
    pSetBkMode.Call(d.Hdc,1)
    pSetTextColor.Call(d.Hdc,fg)
    pDrawText.Call(d.Hdc,uintptr(unsafe.Pointer(str(title))),^uintptr(0),uintptr(unsafe.Pointer(&d.RcItem)),DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func paintOrderedRow(l uintptr) {
    item:=(*drawItem)(unsafe.Pointer(l))
    if item==nil || item.ItemID==^uint32(0) {return}
    i:=int(item.ItemID)
    if i<0 || i>=len(winFiles) {return}
    bg,fg:=winColor(255,255,255),winColor(38,54,77)
    if item.State&ODS_SELECTED!=0 {bg=winColor(221,236,255);fg=winColor(22,82,160)}
    fillColor(item.Hdc,item.RcItem,bg)
    seqBG:=winColor(246,249,253)
    if item.State&ODS_SELECTED!=0 {seqBG=winColor(198,224,255)}
    seq:=winRect{Left:item.RcItem.Left,Top:item.RcItem.Top,Right:item.RcItem.Left+63,Bottom:item.RcItem.Bottom}
    fillColor(item.Hdc,seq,seqBG)
    fillColor(item.Hdc,winRect{seq.Right,item.RcItem.Top,seq.Right+1,item.RcItem.Bottom},winColor(225,234,246))
    pSetBkMode.Call(item.Hdc,1);pSetTextColor.Call(item.Hdc,fg)
    pDrawText.Call(item.Hdc,uintptr(unsafe.Pointer(str(fmt.Sprintf("%02d",i+1)))),^uintptr(0),uintptr(unsafe.Pointer(&seq)),DT_CENTER|DT_VCENTER|DT_SINGLELINE)
    name:=filepath.Base(winFiles[i])
    nameRect:=winRect{seq.Right+13,item.RcItem.Top,item.RcItem.Right-8,item.RcItem.Bottom}
    pDrawText.Call(item.Hdc,uintptr(unsafe.Pointer(str(name))),^uintptr(0),uintptr(unsafe.Pointer(&nameRect)),DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
}
type zipAnalysisResult struct {
    path string
    job *officialJob
    detailed bool
    err error
}

type mergeResult struct {
    checkpoint *diagnosticResult
	path  string
	count int
	err   error
	warning string
}
var (
	hwndMain, hwndList, hwndStatus, hwndMerge, hwndZip, hwndDetail, hwndSaveAs, hwndMixedWarning, hwndProgress, hwndProgressLabel uintptr
	oldListProc                                                    uintptr
	winFiles                                                       []string
	dragFrom                                                       = -1
	isMerging                                                      bool
	resultMu                                                       sync.Mutex
	latestResult                                                   mergeResult
    pendingZipAnalysis zipAnalysisResult
    progressMu sync.Mutex
    latestProgress progressMessage
	fontHandle                                                     uintptr
    titleFont uintptr
    statusBrush uintptr
	windowProc                                                     = syscall.NewCallback(wndProc)
	listProc                                                       = syscall.NewCallback(listWndProc)
)

// Startup diagnostics are disabled by default; no document names or content are logged.
func traceStartup(stage string) {
	path := os.Getenv("FASTPDF_STARTUP_LOG")
	if path == "" { return }
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil { return }
	defer f.Close()
	fmt.Fprintf(f, "%s pid=%d %s\n", time.Now().UTC().Format(time.RFC3339Nano), os.Getpid(), stage)
}

type progressMessage struct {
 percent int
 stage string
}
func setProgressUI(percent int,stage string) {
 if percent<0 {percent=0}
 if percent>100 {percent=100}
 if hwndProgress!=0 {send(hwndProgress,PBM_SETPOS,uintptr(percent),0)}
 if hwndProgressLabel!=0 {
  pSetWindowText.Call(hwndProgressLabel,uintptr(unsafe.Pointer(str(fmt.Sprintf("%d%%",percent)))))
 }
 if stage!="" {status(stage)}
}
func postProgress(percent int,stage string) {
 progressMu.Lock()
 latestProgress=progressMessage{percent,stage}
 progressMu.Unlock()
 pPostMessage.Call(hwndMain,WM_APP+5,0,0)
}
func str(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func fileFilter() []uint16 {
    return windowsFilter("PDF (*.pdf)", "*.pdf")
}
func documentFileFilter() []uint16 {
    return windowsFilter("支援文件（PDF、Word、ODF、Excel、PowerPoint）",
        "*.pdf;*.doc;*.docx;*.docm;*.odt;*.rtf;*.txt;*.xls;*.xlsx;*.xlsm;*.ods;*.csv;*.tsv;*.ppt;*.pptx;*.pptm;*.odp")
}
func windowsFilter(label, pattern string) []uint16 {
    out := make([]uint16, 0, 320)
    for _, text := range []string{label, pattern, "所有檔案 (*.*)", "*.*"} {
        out = append(out, utf16.Encode([]rune(text))...)
        out = append(out, 0)
    }
    return append(out, 0)
}
func send(h uintptr, m uint32, w, l uintptr) uintptr {
	r, _, _ := pSendMessage.Call(h, uintptr(m), w, l)
	return r
}
func status(s string) {
    // The old STATIC control wrapped long paths into adjacent controls.
    // Keep the live status exactly one line even on smaller displays.
    chars := []rune(s)
    if len(chars) > 55 { s = string(chars[:54]) + "…" }
    if hwndStatus != 0 { pSetWindowText.Call(hwndStatus, uintptr(unsafe.Pointer(str(s)))) }
}
func alert(msg string, err bool) {
	flags := uintptr(MB_OK | MB_ICONINFORMATION)
	if err {
		flags = MB_OK | MB_ICONERROR
	}
	pMessageBox.Call(hwndMain, uintptr(unsafe.Pointer(str(msg))), uintptr(unsafe.Pointer(str("快合 PDF"))), flags)
}
// A warning is a user-visible caution, not an antivirus bypass.
// It always runs on the Win32 UI thread, before an output is created.
func confirmMixedMerge(count int, meetsThreshold bool) bool {
    if !meetsThreshold || hwndMixedWarning==0 || send(hwndMixedWarning,BM_GETCHECK,0,0)==0 {
        return true
    }
    accepted:=confirmModernMixedMerge(count)
    if !accepted {
        setProgressUI(0,"已停止混合格式合併，未建立輸出。")
        return false
    }
    return true
}

func updateWarningPreference() {
    enabled:=send(hwndMixedWarning,BM_GETCHECK,0,0)!=0
    previous:=readMixedWarningSetting()
    if err:=writeMixedWarningSetting(enabled);err!=nil {
        if previous {send(hwndMixedWarning,BM_SETCHECK,1,0)} else {send(hwndMixedWarning,BM_SETCHECK,0,0)}
        alert("無法儲存慎用警告設定：\n"+err.Error()+"\n設定已恢復原值。",true)
        return
    }
    if enabled {status("已開啟混合格式多檔慎用警告（設定已記憶）。")
    } else {status("已關閉混合格式多檔慎用警告（設定已記憶）。")}
}

func makeChild(class, text string, x, y, w, h int, style uint32, id int) uintptr {
	r, _, _ := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(str(class))), uintptr(unsafe.Pointer(str(text))), uintptr(WS_CHILD|WS_VISIBLE|style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), hwndMain, uintptr(id), 0, 0)
	if fontHandle != 0 {
		send(r, WM_SETFONT, fontHandle, 1)
	}
	return r
}


func initUI() {
    initClasses:=[2]uint32{8,ICC_PROGRESS_CLASS}
    pInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&initClasses[0])))
    fontHeight := int32(-18)
    fontHandle, _, _ = pCreateFont.Call(uintptr(fontHeight),0,0,0,400,0,0,0,1,0,0,0,0,uintptr(unsafe.Pointer(str("Microsoft JhengHei UI"))))
    titleHeight := int32(-29)
    titleFont,_,_=pCreateFont.Call(uintptr(titleHeight),0,0,0,700,0,0,0,1,0,0,0,0,uintptr(unsafe.Pointer(str("Microsoft JhengHei UI"))))
    header:=makeChild("STATIC","公文快合",32,17,280,39,0,0)
    makeChild("STATIC","臺北市政府地政局測繪科開發",354,35,308,26,SS_LEFTNOWORDWRAP|SS_NOPREFIX,0)
    if titleFont != 0 {send(header, WM_SETFONT,titleFont,1)}
    makeChild("STATIC","本機處理 · 無須上傳 · ZIP 即放即合併",33,66,590,27,0,0)
    makeChild("STATIC","公文 ZIP　自動合併",43,124,575,34,0,0)
    makeChild("STATIC","PDF、Word、ODF、Excel、PowerPoint 附件自動轉檔並依序合併",43,161,594,30,0,0)
    hwndZip=makeChild("BUTTON","選擇公文 ZIP",43,201,397,49,WS_TABSTOP|BS_OWNERDRAW,ID_ADD_ZIP)
    makeChild("BUTTON","分段接續",448,201,199,49,WS_TABSTOP|BS_OWNERDRAW,ID_CHECKPOINT_ZIP)
    hwndDetail=makeChild("BUTTON","使用完整日期與發文字號命名",45,261,582,27,WS_TABSTOP|0x00000003,ID_FULL_NAME)
    makeChild("STATIC","一般文件　手動排序合併",42,333,335,28,0,0)
    // Default is zero-dialog output in the first file's directory.
    // Opt-in checkbox preserves the previous Save As picker on demand.
    hwndSaveAs=makeChild("BUTTON","合併時開啟另存視窗",414,329,230,31,WS_TABSTOP|0x00000003,ID_SAVE_AS)
    makeChild("STATIC","序號",51,366,60,23,SS_LEFTNOWORDWRAP,ID_COLUMN_ORDER)
    makeChild("STATIC","檔案名稱",119,366,342,23,SS_LEFTNOWORDWRAP,ID_COLUMN_FILENAME)
    hwndList=makeChild("LISTBOX","",42,391,437,120,WS_BORDER|WS_VSCROLL|WS_TABSTOP|LBS_NOTIFY|LBS_OWNERDRAWFIXED|LBS_HASSTRINGS|LBS_NOINTEGRALHEIGHT,ID_LIST)
    oldListProc,_,_=pSetWindowLongPtr.Call(hwndList,GWLP_WNDPROC,listProc)
    makeChild("BUTTON","加入文件",491,370,154,32,WS_TABSTOP|BS_OWNERDRAW,ID_ADD)
    makeChild("BUTTON","上移",491,410,73,32,WS_TABSTOP|BS_OWNERDRAW,ID_UP)
    makeChild("BUTTON","下移",572,410,73,32,WS_TABSTOP|BS_OWNERDRAW,ID_DOWN)
    makeChild("BUTTON","移除",491,451,73,32,WS_TABSTOP|BS_OWNERDRAW,ID_REMOVE)
    makeChild("BUTTON","清空",572,451,73,32,WS_TABSTOP|BS_OWNERDRAW,ID_CLEAR)
    hwndMerge=makeChild("BUTTON","合併一般文件",42,524,603,40,WS_TABSTOP|BS_OWNERDRAW,ID_MERGE)
    hwndMixedWarning=makeChild("BUTTON","混合格式超過3份時顯示慎用警告（適用所有合併方式）",39,584,605,30,WS_TABSTOP|0x00000003,ID_MIXED_WARNING)
    if readMixedWarningSetting() {send(hwndMixedWarning,BM_SETCHECK,1,0)}
    statusBrush,_,_=pCreateSolidBrush.Call(winColor(236,242,249))
    hwndStatus=makeChild("STATIC","待命｜拖入 ZIP 即自動產出 PDF",38,631,606,24,SS_LEFTNOWORDWRAP|SS_ENDELLIPSIS|SS_NOPREFIX,ID_STATUS)
    hwndProgress=makeChild("msctls_progress32","",40,660,551,19,0,ID_PROGRESS)
    if hwndProgress!=0 {send(hwndProgress,PBM_SETRANGE32,0,100)}
    hwndProgressLabel=makeChild("STATIC","0%",600,657,52,24,SS_LEFTNOWORDWRAP|SS_NOPREFIX,ID_PROGRESS_LABEL)
    pDragAcceptFiles.Call(hwndMain,1)
    pDragAcceptFiles.Call(hwndList,1)
}

func refresh(selectIndex int) {
	send(hwndList, LB_RESETCONTENT, 0, 0)
	for _, file := range winFiles {
		label := filepath.Base(file)
		send(hwndList, LB_ADDSTRING, 0, uintptr(unsafe.Pointer(str(label))))
	}
	if selectIndex >= 0 && selectIndex < len(winFiles) {
		send(hwndList, LB_SETCURSEL, uintptr(selectIndex), 0)
	}
	if len(winFiles) < 2 {
		status(fmt.Sprintf("共 %d 份文件；可加入 PDF 或 Office／ODF 文件。", len(winFiles)))
	} else {
		status(fmt.Sprintf("目前 %d 份文件；清單即合併順序。", len(winFiles)))
	}
}
func selected() int {
	n := int(int32(send(hwndList, LB_GETCURSEL, 0, 0)))
	if n < 0 || n >= len(winFiles) {
		return -1
	}
	return n
}
func moveItem(a, b int) {
	if a < 0 || a >= len(winFiles) || b < 0 || b >= len(winFiles) || a == b {
		return
	}
	v := winFiles[a]
	winFiles = append(winFiles[:a], winFiles[a+1:]...)
	if b >= len(winFiles) {
		winFiles = append(winFiles, v)
	} else {
		winFiles = append(winFiles, "")
		copy(winFiles[b+1:], winFiles[b:])
		winFiles[b] = v
	}
	refresh(b)
}
func appendFiles(files []string) {
	have := map[string]bool{}
	for _, f := range winFiles {
		have[strings.ToLower(f)] = true
	}
	count := 0
	for _, f := range files {
		f = filepath.Clean(f)
		if !supportedOfficialExt(filepath.Ext(f)) {
			continue
		}
		abs, e := filepath.Abs(f)
		if e != nil {
			continue
		}
		if _, e = os.Stat(abs); e != nil {
			continue
		}
		if have[strings.ToLower(abs)] {
			continue
		}
		have[strings.ToLower(abs)] = true
		winFiles = append(winFiles, abs)
		count++
	}
	refresh(-1)
	if count == 0 {
		status("沒有新增支援文件；重複或不支援的檔案已略過。")
		return
	}
	status(fmt.Sprintf("已加入 %d 份文件。請在清單中調整順序。", count))
}
func dropFiles(hdrop uintptr) {
	count, _, _ := pDragQueryFile.Call(hdrop, ^uintptr(0), 0, 0)
	names := make([]string, 0, count)
	for i := uintptr(0); i < count; i++ {
		n, _, _ := pDragQueryFile.Call(hdrop, i, 0, 0)
		if n > 32767 {
			continue
		}
		buf := make([]uint16, n+2)
		pDragQueryFile.Call(hdrop, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		names = append(names, syscall.UTF16ToString(buf))
	}
	pDragFinish.Call(hdrop)
	handleInputFiles(names)
}
// ZIP exports use a separate workflow rather than adding opinion sheets to the manual list.
func handleInputFiles(files []string) {
    traceStartup("handleInputFiles begin")
    var zips, docs []string
    for _, p := range files {
        ext := strings.ToLower(filepath.Ext(p))
        if ext == ".zip" { zips = append(zips, p) } else if supportedOfficialExt(ext) { docs = append(docs, p) }
    }
    if len(zips) > 0 {
        if len(zips) != 1 || len(docs) > 0 {
            alert("請一次只拖入一份公文 ZIP，或另外拖入一般文件。",true)
            return
        }
        startOfficialZIP(zips[0])
        return
    }
    if len(docs) > 0 { appendFiles(docs) }
}

func handleStartupFiles(args []string) {
    // Dragging files onto an unopened EXE launches it with file arguments.
    // That workflow is unattended and never invokes GetSaveFileNameW.
    if len(args) == 0 { return }
    if args[0] == "--manual" {
        handleInputFiles(args[1:])
        return
    }
    var zips, docs []string
    for _, p := range args {
        ext := strings.ToLower(filepath.Ext(p))
        if ext == ".zip" { zips = append(zips, p); continue }
        if !supportedOfficialExt(ext) {
            status("不支援的文件：" + filepath.Base(p))
            return
        }
        abs, err := filepath.Abs(p)
        if err != nil { status("無法取得檔案路徑"); return }
        if st, err := os.Stat(abs); err != nil || st.IsDir() {
            status("檔案不存在或不是一般文件：" + filepath.Base(p))
            return
        }
        docs = append(docs, abs)
    }
    if len(zips) == 1 && len(docs) == 0 { startOfficialZIP(zips[0]); return }
    if len(zips) > 0 { status("ZIP 不可與一般文件混合拖入，請分開處理。"); return }
    if len(docs) > 0 { startAutoMixed(docs) }
}

func startAutoMixed(files []string) {
    if isMerging { return }
    if !confirmMixedMerge(len(files),needsMixedFileWarning(files)) { return }
    output, err := autoMergedPath(files)
    if err != nil { status(err.Error()); return }
    startMixedWorker(files, output)
}

func startMixedWorker(files []string, output string) {
    if isMerging { return }
    isMerging = true
    setProgressUI(3,fmt.Sprintf("開始處理%d份文件…",len(files)))
    snapshot := append([]string(nil), files...)
    go func() {
        result := mergeResult{path: output}
        defer func() {
            if v := recover(); v != nil { result.err = fmt.Errorf("合併作業異常：%v", v) }
            resultMu.Lock()
            latestResult = result
            resultMu.Unlock()
            pPostMessage.Call(hwndMain, WM_APP+1, 0, 0)
        }()
        result.count, result.err = mergeMixedFilesWithProgress(snapshot,output,postProgress)
    }()
}
func zipFilter() []uint16 {
	return []uint16{'Z', 'I', 'P', ' ', '(', '*', '.', 'z', 'i', 'p', ')', 0, '*', '.', 'z', 'i', 'p', 0, 0}
}
// Manual checkpoint workflow keeps each Office COM session bounded to two
// conversions. No relaunch, bypass, or interference with endpoint protection.
func chooseCheckpointZIP() {
    buf:=make([]uint16,32768)
    filter:=zipFilter()
    f:=openFileName{lStructSize:uint32(unsafe.Sizeof(openFileName{})),hwndOwner:hwndMain,
        lpstrFilter:&filter[0],lpstrFile:&buf[0],nMaxFile:uint32(len(buf)),
        lpstrTitle:str("選擇公文 ZIP 進行分段接續"),
        flags:OFN_EXPLORER|OFN_FILEMUSTEXIST|OFN_NOCHANGEDIR}
    ret,_,_:=pGetOpenFileName.Call(uintptr(unsafe.Pointer(&f)))
    if ret!=0 {startCheckpointZIP(syscall.UTF16ToString(buf))}
}
func startCheckpointZIP(path string) {
    if isMerging{return}
    isMerging=true
    setProgressUI(2,"分段接續：每次最多轉換 2 份 Office 附件")
    go func(){
        result:=mergeResult{}
        defer func(){
            if v:=recover();v!=nil {result.err=fmt.Errorf("分段接續異常：%v",v)}
            resultMu.Lock()
            latestResult=result
            resultMu.Unlock()
            pPostMessage.Call(hwndMain,WM_APP+1,0,0)
        }()
        stage,err:=runZIPDiagnosticStage(path)
        result.err=err
        if err==nil {result.checkpoint=&stage}
    }()
}
func chooseZIP() {
	buf := make([]uint16, 32768)
	filter := zipFilter()
	f := openFileName{lStructSize: uint32(unsafe.Sizeof(openFileName{})), hwndOwner: hwndMain,
		lpstrFilter: &filter[0], lpstrFile: &buf[0], nMaxFile: uint32(len(buf)),
		lpstrTitle: str("選擇公文打包 ZIP"), flags: OFN_EXPLORER | OFN_FILEMUSTEXIST | OFN_NOCHANGEDIR}
	ret, _, _ := pGetOpenFileName.Call(uintptr(unsafe.Pointer(&f)))
	if ret != 0 {
		startOfficialZIP(syscall.UTF16ToString(buf))
	}
}
// ZIP mode is intentionally zero-dialog: analysis, naming, extraction and PDF
// merging all run off the GUI thread. The resulting PDF stays beside its ZIP.
func autoOutputPath(zipPath, name string) (string, error) {
	folder := filepath.Dir(zipPath)
	if cleanComponent(name) == "" || strings.ContainsAny(name, "\\/:*?\"<>|") {
		return "", fmt.Errorf("不安全的輸出檔名")
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	for i := 1; i <= 10000; i++ {
		candidate := name
		if i > 1 { candidate = fmt.Sprintf("%s (%d).pdf", stem, i) }
		out := filepath.Join(folder, candidate)
		_, err := os.Stat(out)
		if os.IsNotExist(err) { return out, nil }
		if err != nil { return "", err }
	}
	return "", fmt.Errorf("輸出檔名編號已達上限")
}
func startOfficialZIP(path string) {
    if isMerging { return }
    detailed:=send(hwndDetail,BM_GETCHECK,0,0)!=0
    traceStartup("ZIP analysis scheduled")
    isMerging=true
    setProgressUI(2,"正在分析公文ZIP："+filepath.Base(path))
    go func() {
        job,err:=analyzeZIP(path)
        resultMu.Lock()
        pendingZipAnalysis=zipAnalysisResult{path:path,detailed:detailed,job:job,err:err}
        resultMu.Unlock()
        pPostMessage.Call(hwndMain,WM_APP+4,0,0)
    }()
}

func runAnalyzedZIP(request zipAnalysisResult) {
    setProgressUI(7,"正在準備合併："+filepath.Base(request.path))
    go func(){
        var result mergeResult
        defer func(){
            if v:=recover();v!=nil{result.err=fmt.Errorf("公文ZIP處理異常：%v",v)}
            resultMu.Lock()
            latestResult=result
            resultMu.Unlock()
            pPostMessage.Call(hwndMain,WM_APP+1,0,0)
        }()
        job:=request.job
        name,nameErr:=job.outputName(request.detailed)
        if nameErr!=nil {
            fallback:=cleanComponent(strings.TrimSuffix(filepath.Base(request.path),filepath.Ext(request.path)))
            if fallback==""{fallback="來文"}
            name=fallback+"_來文資料待確認.pdf"
        }
        out,err:=autoOutputPath(request.path,name)
        if err!=nil{result.err=err;return}
        result.path=out
        result.count,result.err=mergeOfficialZIPWithProgress(job,out,postProgress)
        if result.err==nil&&nameErr!=nil{result.warning="（檔名待核對）"}
    }()
}

func chooseFiles() {
	data := make([]uint16, 32768)
	filter := documentFileFilter()
	f := openFileName{lStructSize: uint32(unsafe.Sizeof(openFileName{})), hwndOwner: hwndMain,
		lpstrFilter: &filter[0], lpstrFile: &data[0], nMaxFile: uint32(len(data)),
		lpstrTitle: str("加入 PDF 或 Office／ODF 文件"), flags: OFN_EXPLORER | OFN_ALLOWMULTISELECT | OFN_FILEMUSTEXIST | OFN_NOCHANGEDIR}
	r, _, _ := pGetOpenFileName.Call(uintptr(unsafe.Pointer(&f)))
	if r == 0 {
		return
	}
	var sections []string
	start := 0
	for i, c := range data {
		if c != 0 {
			continue
		}
		if i == start {
			break
		}
		sections = append(sections, string(utf16.Decode(data[start:i])))
		start = i + 1
	}
	if len(sections) == 0 {
		return
	}
	if len(sections) == 1 {
		appendFiles(sections)
		return
	}
	var names []string
	for _, name := range sections[1:] {
		names = append(names, filepath.Join(sections[0], name))
	}
	appendFiles(names)
}
func chooseOutput() string {
	if len(winFiles) < 2 {
		alert("請至少加入 2 份文件，才能手動合併。", true)
		return ""
	}
	folder := filepath.Dir(winFiles[0])
	base := "合併檔案.pdf"
	for n := 2; n < 9999; n++ {
		if _, e := os.Stat(filepath.Join(folder, base)); os.IsNotExist(e) {
			break
		}
		base = fmt.Sprintf("合併檔案 (%d).pdf", n)
	}
	raw := []rune(base)
	fileName := make([]uint16, 32768)
	copy(fileName, utf16.Encode(raw))
	filter := fileFilter()
	f := openFileName{lStructSize: uint32(unsafe.Sizeof(openFileName{})), hwndOwner: hwndMain,
		lpstrFilter: &filter[0], lpstrFile: &fileName[0], nMaxFile: uint32(len(fileName)),
		lpstrInitialDir: str(folder), lpstrTitle: str("儲存合併後的 PDF"), lpstrDefExt: str("pdf"), flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_NOCHANGEDIR}
	r, _, _ := pGetSaveFileName.Call(uintptr(unsafe.Pointer(&f)))
	if r == 0 {
		return ""
	}
	path := syscall.UTF16ToString(fileName)
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		path += ".pdf"
	}
	if _, e := os.Stat(path); e == nil {
		alert("此檔案已存在，請改用其他檔名（不會覆寫原有 PDF）。", true)
		return ""
	}
	return path
}
func startMerge() {
    if isMerging { return }
    if len(winFiles) < 2 { status("請先加入至少 2 份文件。"); return }
    if !confirmMixedMerge(len(winFiles),needsMixedFileWarning(winFiles)) { return }
    var out string
    if send(hwndSaveAs, BM_GETCHECK, 0, 0) != 0 {
        out = chooseOutput()
        if out == "" { return }
    } else {
        var err error
        out, err = autoMergedPath(winFiles)
        if err != nil {
            alert("無法取得預設儲存路徑：\n"+err.Error(), true)
            return
        }
    }
    startMixedWorker(winFiles, out)
}
func removeSelected() {
	i := selected()
	if i < 0 {
		return
	}
	winFiles = append(winFiles[:i], winFiles[i+1:]...)
	if i == len(winFiles) {
		i--
	}
	refresh(i)
}
func handleCommand(id int) {
    if id==ID_MIXED_WARNING {
        updateWarningPreference()
        return
    }
	if isMerging {
		return
	}
	switch id {
	case ID_ADD:
		chooseFiles()
	case ID_UP:
		i := selected()
		moveItem(i, i-1)
	case ID_DOWN:
		i := selected()
		moveItem(i, i+1)
	case ID_REMOVE:
		removeSelected()
	case ID_CLEAR:
		winFiles = nil
		refresh(-1)
	case ID_MERGE:
		startMerge()
	case ID_ADD_ZIP:
		chooseZIP()
    case ID_CHECKPOINT_ZIP:
        chooseCheckpointZIP()
	}
}
func listIndexFromPoint(l uintptr) int {
	ret := send(hwndList, LB_ITEMFROMPOINT, 0, l)
	if ret>>16 != 0 {
		return -1
	}
	i := int(ret & 0xFFFF)
	if i < 0 || i >= len(winFiles) {
		return -1
	}
	return i
}
func listWndProc(h uintptr, m uint32, w, l uintptr) uintptr {
	switch m {
	case WM_DROPFILES:
		if !isMerging {
			dropFiles(w)
		} else {
			pDragFinish.Call(w)
		}
		return 0
	case WM_LBUTTONDOWN:
		dragFrom = listIndexFromPoint(l)
	case WM_LBUTTONUP:
		to := listIndexFromPoint(l)
		if dragFrom >= 0 && to >= 0 && dragFrom != to && !isMerging {
			moveItem(dragFrom, to)
			dragFrom = -1
			return 0
		}
		dragFrom = -1
	case WM_KEYDOWN:
		if w == VK_DELETE {
			removeSelected()
			return 0
		}
		state, _, _ := pGetKeyState.Call(VK_CONTROL)
		if state&0x8000 != 0 {
			i := selected()
			if w == VK_UP {
				moveItem(i, i-1)
				return 0
			}
			if w == VK_DOWN {
				moveItem(i, i+1)
				return 0
			}
		}
	}
	r, _, _ := pCallWindowProc.Call(oldListProc, h, uintptr(m), w, l)
	return r
}
func wndProc(h uintptr, m uint32, w, l uintptr) uintptr {
	switch m {
	case WM_ERASEBKGND:
        return 1
    case WM_PAINT:
        paintBackground(h)
        return 0
    case WM_DRAWITEM:
        item:=(*drawItem)(unsafe.Pointer(l))
        if item!=nil && item.CtlType==ODT_LISTBOX && item.CtlID==ID_LIST {
            paintOrderedRow(l)
        } else {paintButton(l)}
        return 1
    case WM_MEASUREITEM:
        item:=(*measureItem)(unsafe.Pointer(l))
        if item!=nil && item.ctlID==ID_LIST {
            item.itemHeight=24
            return 1
        }
    case WM_CTLCOLORSTATIC:
        if (l == hwndStatus || l == hwndProgressLabel) && statusBrush != 0 {
            // Paint over the old status before rendering new text. Transparent
            // static controls can leave previous long filenames behind.
            pSetBkMode.Call(w,2)
            pSetBkColor.Call(w,winColor(236,242,249))
            pSetTextColor.Call(w,winColor(46,65,92))
            return statusBrush
        }
        pSetBkMode.Call(w,1)
        pSetTextColor.Call(w,winColor(46,65,92))
        b,_,_:=pGetStockObject.Call(5)
        return b
    case WM_CREATE:
		traceStartup("WM_CREATE received")
		hwndMain = h
		initUI()
		traceStartup("WM_CREATE UI initialized")
		return 0
	case WM_COMMAND:
		id := int(w & 0xFFFF)
		if (id == ID_ADD || id == ID_UP || id == ID_DOWN || id == ID_REMOVE || id == ID_CLEAR || id == ID_MERGE || id == ID_ADD_ZIP || id == ID_MIXED_WARNING || id == ID_CHECKPOINT_ZIP) {
			handleCommand(id)
			return 0
		}
	case WM_DROPFILES:
		if !isMerging {
			dropFiles(w)
		} else {
			pDragFinish.Call(w)
		}
		return 0
	case WM_APP + 3:
		if len(os.Args) > 1 { handleStartupFiles(os.Args[1:]) }
		return 0
    case WM_APP + 5:
        progressMu.Lock()
        update:=latestProgress
        progressMu.Unlock()
        if isMerging {setProgressUI(update.percent,update.stage)}
        return 0
    case WM_APP + 4:
        // ZIP analysis completes on a worker, but the modal confirmation must
        // run on the UI thread before Office conversion or PDF output begins.
        resultMu.Lock()
        request:=pendingZipAnalysis
        pendingZipAnalysis=zipAnalysisResult{}
        resultMu.Unlock()
        if request.err!=nil {
            isMerging=false
            setProgressUI(0,"ZIP分析失敗："+request.err.Error())
            alert("公文ZIP分析失敗：\n"+request.err.Error(),true)
            return 0
        }
        if request.job==nil {
            isMerging=false
            setProgressUI(0,"公文ZIP分析沒有產生有效結果")
            alert("公文ZIP分析沒有產生有效結果。",true)
            return 0
        }
        if !confirmMixedMerge(len(request.job.Entries),needsMixedZIPWarning(request.job.Entries)) {
            isMerging=false
            setProgressUI(0,"已取消合併")
            return 0
        }
        runAnalyzedZIP(request)
        return 0
	case WM_APP + 1:
		traceStartup("WM_APP+1 received")
		resultMu.Lock()
		result := latestResult
		resultMu.Unlock()
		traceStartup("WM_APP+1 result obtained")
		isMerging = false
		traceStartup("WM_APP+1 flags restored")
		if result.checkpoint!=nil {
            d:=result.checkpoint
            if d.Finished {
                setProgressUI(100,fmt.Sprintf("分段接續完成：%d頁｜%s",d.Pages,filepath.Base(d.PDFPath)))
            } else {
                percent:=d.Completed*100/d.Total
                setProgressUI(percent,fmt.Sprintf("第%d階段完成：%d／%d份｜請重新開啟並選取相同ZIP",d.Stage,d.Completed,d.Total))
            }
            pSetTimer.Call(hwndMain,checkpointExitTimer,2200,0)
            return 0
        }
        if result.err != nil {
			traceStartup("WM_APP+1 failure update begin")
			setProgressUI(0,"合併失敗｜"+result.err.Error())
			alert("合併失敗：\n"+result.err.Error(), true)
		} else {
			traceStartup("WM_APP+1 success update begin")
			setProgressUI(100,fmt.Sprintf("完成%d頁%s｜%s",result.count,result.warning,filepath.Base(result.path)))
		}
		traceStartup("WM_APP+1 done")
		return 0
	case WM_TIMER:
        if w==checkpointExitTimer {
            pKillTimer.Call(hwndMain,checkpointExitTimer)
            pDestroyWindow.Call(hwndMain)
            return 0
        }
    case WM_DESTROY:
        if statusBrush != 0 { pDeleteObject.Call(statusBrush); statusBrush = 0 }
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProc.Call(h, uintptr(m), w, l)
	return r
}
func main() {
	// Win32 windows and their GetMessage loop must stay on the same OS thread.
	// Go may otherwise migrate this goroutine and leave the original window unresponsive.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	traceStartup("main entry")
	pSetProcessDPIAware.Call()
	instance, _, _ := pGetModuleHandle.Call(0)
	className := str("FastPDFMergeNativeGUI")
	icon,_,_:=pLoadIcon.Call(instance,1)
    wc := wndClass{cbSize: uint32(unsafe.Sizeof(wndClass{})), lpfnWndProc: windowProc, hInstance: instance, hbrBackground: 6, lpszClassName: className,hIcon: icon,hIconSm: icon}
	atom, _, e := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	traceStartup(fmt.Sprintf("RegisterClassExW atom=%d error=%v", atom, e))
	if atom == 0 {
		pMessageBox.Call(0, uintptr(unsafe.Pointer(str("無法註冊 Windows 視窗類別："+e.Error()))), uintptr(unsafe.Pointer(str("公文 PDF 快合｜啟動失敗"))), MB_OK|MB_ICONERROR)
		return
	}
	screenX, _, _ := pGetSystemMetrics.Call(0)
    screenY, _, _ := pGetSystemMetrics.Call(1)
    // Center inside the usable desktop, not behind the Windows taskbar.
    work:=winRect{0,0,int32(screenX),int32(screenY)}
    ok,_,_:=pSystemParametersInfo.Call(0x0030,0,uintptr(unsafe.Pointer(&work)),0) // SPI_GETWORKAREA
    if ok==0 {work=winRect{0,0,int32(screenX),int32(screenY)}}
    x:=int(work.Left)+(int(work.Right-work.Left)-690)/2
    y:=int(work.Top)+(int(work.Bottom-work.Top)-728)/2
    if x<int(work.Left) {x=int(work.Left)}
    if y<int(work.Top) {y=int(work.Top)}
	traceStartup("CreateWindowExW begin")
	hwnd, _, windowErr := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(str("公文 PDF 快合 v1.8.0｜臺北市政府地政局測繪科開發"))), uintptr(WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX|WS_VISIBLE), uintptr(x), uintptr(y), 690, 728, 0, 0, instance, 0)
	traceStartup(fmt.Sprintf("CreateWindowExW hwnd=%d error=%v", hwnd, windowErr))
	if hwnd == 0 {
		pMessageBox.Call(0, uintptr(unsafe.Pointer(str("無法建立程式視窗："+windowErr.Error()))), uintptr(unsafe.Pointer(str("公文 PDF 快合｜啟動失敗"))), MB_OK|MB_ICONERROR)
		return
	}
	if len(os.Args) > 1 { pPostMessage.Call(hwndMain, WM_APP+3, 0, 0) }
	traceStartup("enter message loop")
	var m msg
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}
