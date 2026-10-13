//go:build windows

package main

import (
	"fmt"
	"os"
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
	pEnableWindow             = u32.NewProc("EnableWindow")
	pDestroyWindow            = u32.NewProc("DestroyWindow")
	pPostQuitMessage          = u32.NewProc("PostQuitMessage")
	pSetWindowLongPtr         = u32.NewProc("SetWindowLongPtrW")
	pCallWindowProc           = u32.NewProc("CallWindowProcW")
	pGetSystemMetrics         = u32.NewProc("GetSystemMetrics")
	pGetModuleHandle          = k32.NewProc("GetModuleHandleW")
	pDragAcceptFiles          = s32.NewProc("DragAcceptFiles")
	pDragQueryFile            = s32.NewProc("DragQueryFileW")
	pDragFinish               = s32.NewProc("DragFinish")
	pGetOpenFileName          = comdlg.NewProc("GetOpenFileNameW")
	pGetSaveFileName          = comdlg.NewProc("GetSaveFileNameW")
	pCreateFont               = g32.NewProc("CreateFontW")
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
	LBS_NOTIFY           = 0x0001
	LBS_NOINTEGRALHEIGHT = 0x0100
	WM_CREATE            = 0x0001
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
	IDYES                = 6
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
	ID_LIST              = 201
	ID_STATUS            = 202
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
type mergeResult struct {
	path  string
	count int
	err   error
	warning string
}
var (
	hwndMain, hwndList, hwndStatus, hwndMerge, hwndZip, hwndDetail uintptr
	oldListProc                                                    uintptr
	winFiles                                                       []string
	dragFrom                                                       = -1
	isMerging                                                      bool
	resultMu                                                       sync.Mutex
	latestResult                                                   mergeResult
	fontHandle                                                     uintptr
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

func str(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func fileFilter() []uint16 {
	return []uint16{'P', 'D', 'F', ' ', '(', '*', '.', 'p', 'd', 'f', ')', 0, '*', '.', 'p', 'd', 'f', 0, 0}
}
func send(h uintptr, m uint32, w, l uintptr) uintptr {
	r, _, _ := pSendMessage.Call(h, uintptr(m), w, l)
	return r
}
func status(s string) { pSetWindowText.Call(hwndStatus, uintptr(unsafe.Pointer(str(s)))) }
func alert(msg string, err bool) {
	flags := uintptr(MB_OK | MB_ICONINFORMATION)
	if err {
		flags = MB_OK | MB_ICONERROR
	}
	pMessageBox.Call(hwndMain, uintptr(unsafe.Pointer(str(msg))), uintptr(unsafe.Pointer(str("快合 PDF"))), flags)
}
func makeChild(class, text string, x, y, w, h int, style uint32, id int) uintptr {
	r, _, _ := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(str(class))), uintptr(unsafe.Pointer(str(text))), uintptr(WS_CHILD|WS_VISIBLE|style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), hwndMain, uintptr(id), 0, 0)
	if fontHandle != 0 {
		send(r, WM_SETFONT, fontHandle, 1)
	}
	return r
}


func initUI() {
	fontHeight := int32(-18)
	fontHandle, _, _ = pCreateFont.Call(uintptr(fontHeight), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(str("Microsoft JhengHei UI"))))
	makeChild("STATIC", "公文 ZIP｜拖入即合併", 22, 17, 550, 31, 0, 0)
	makeChild("STATIC", "自動整理來文及 ATTCH 附件，結果存回 ZIP 相同資料夾。", 22, 52, 608, 25, 0, 0)
	hwndZip = makeChild("BUTTON", "選擇公文 ZIP…", 22, 91, 600, 51, WS_TABSTOP, ID_ADD_ZIP)
	hwndDetail = makeChild("BUTTON", "以完整日期、文號命名", 22, 153, 600, 31, WS_TABSTOP|0x00000003, ID_FULL_NAME)
	makeChild("STATIC", "一般 PDF｜手動排序（不讀取桌面圖示）", 22, 211, 595, 27, 0, 0)
	hwndList = makeChild("LISTBOX", "", 22, 241, 446, 184, WS_BORDER|WS_VSCROLL|WS_TABSTOP|LBS_NOTIFY|LBS_NOINTEGRALHEIGHT, ID_LIST)
	oldListProc, _, _ = pSetWindowLongPtr.Call(hwndList, GWLP_WNDPROC, listProc)
	makeChild("BUTTON", "加入 PDF…", 480, 241, 142, 34, WS_TABSTOP, ID_ADD)
	makeChild("BUTTON", "↑ 上移", 480, 281, 142, 31, WS_TABSTOP, ID_UP)
	makeChild("BUTTON", "↓ 下移", 480, 318, 142, 31, WS_TABSTOP, ID_DOWN)
	makeChild("BUTTON", "移除", 480, 355, 142, 31, WS_TABSTOP, ID_REMOVE)
	makeChild("BUTTON", "清空", 480, 392, 142, 31, WS_TABSTOP, ID_CLEAR)
	hwndMerge = makeChild("BUTTON", "合併一般 PDF…", 22, 441, 600, 41, WS_TABSTOP, ID_MERGE)
	hwndStatus = makeChild("STATIC", "待命：將公文 ZIP 拖入視窗，即自動輸出。", 22, 499, 610, 45, 0, ID_STATUS)
	pDragAcceptFiles.Call(hwndMain, 1)
	pDragAcceptFiles.Call(hwndList, 1)
}

func refresh(selectIndex int) {
	send(hwndList, LB_RESETCONTENT, 0, 0)
	for i, file := range winFiles {
		label := fmt.Sprintf("%02d  %s", i+1, filepath.Base(file))
		send(hwndList, LB_ADDSTRING, 0, uintptr(unsafe.Pointer(str(label))))
	}
	if selectIndex >= 0 && selectIndex < len(winFiles) {
		send(hwndList, LB_SETCURSEL, uintptr(selectIndex), 0)
	}
	if len(winFiles) < 2 {
		status(fmt.Sprintf("共 %d 份 PDF，至少需 2 份才能合併。", len(winFiles)))
	} else {
		status(fmt.Sprintf("目前 %d 份 PDF；清單即合併順序。", len(winFiles)))
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
		if !strings.EqualFold(filepath.Ext(f), ".pdf") {
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
		status("沒有新增 PDF：請拖入 PDF 檔案（重複項目會略過）。")
		return
	}
	status(fmt.Sprintf("已加入 %d 份 PDF。請在清單中調整合併順序。", count))
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
	var zips []string
	var pdfs []string
	for _, p := range files {
		switch strings.ToLower(filepath.Ext(p)) {
		case ".zip":
			zips = append(zips, p)
		case ".pdf":
			pdfs = append(pdfs, p)
		}
	}
	if len(zips) > 0 {
		traceStartup("handleInputFiles has ZIP")
		if len(zips) != 1 || len(pdfs) > 0 {
			alert("請一次拖入一份公文 ZIP；一般 PDF 請另外拖入。", true)
			return
		}
		startOfficialZIP(zips[0])
		return
	}
	if len(pdfs) > 0 { appendFiles(pdfs) }
}
func zipFilter() []uint16 {
	return []uint16{'Z', 'I', 'P', ' ', '(', '*', '.', 'z', 'i', 'p', ')', 0, '*', '.', 'z', 'i', 'p', 0, 0}
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
	detailed := send(hwndDetail, BM_GETCHECK, 0, 0) != 0
	traceStartup("ZIP job started")
	isMerging = true
	status("正在處理：" + filepath.Base(path))
	go func() {
		var result mergeResult
		defer func() {
			if v := recover(); v != nil { result.err = fmt.Errorf("公文 ZIP 處理異常：%v", v) }
			resultMu.Lock()
			latestResult = result
			resultMu.Unlock()
			pPostMessage.Call(hwndMain, WM_APP+1, 0, 0)
			traceStartup("ZIP notification posted")
		}()
		traceStartup("ZIP analysis begin")
		job, err := analyzeZIP(path)
		if err != nil { traceStartup("ZIP analysis failed: "+err.Error()); result.err = err; return }
		traceStartup(fmt.Sprintf("ZIP analysis done: %d entries", len(job.Entries)))
		name, nameErr := job.outputName(detailed)
		if nameErr != nil {
			fallback := cleanComponent(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
			if fallback == "" { fallback = "來文" }
			name = fallback + "_來文資料待確認.pdf"
		}
		out, err := autoOutputPath(path, name)
		if err != nil { result.err = err; return }
		result.path = out
		traceStartup("ZIP merge begin")
		result.count, result.err = mergeOfficialZIP(job, out)
		if result.err != nil { traceStartup("ZIP merge failed: "+result.err.Error()) } else { traceStartup("ZIP merge done") }
		if result.err == nil && nameErr != nil { result.warning = "（檔名待核對）" }
	}()
}

func chooseFiles() {
	data := make([]uint16, 32768)
	filter := fileFilter()
	f := openFileName{lStructSize: uint32(unsafe.Sizeof(openFileName{})), hwndOwner: hwndMain,
		lpstrFilter: &filter[0], lpstrFile: &data[0], nMaxFile: uint32(len(data)),
		lpstrTitle: str("加入 PDF 檔案"), flags: OFN_EXPLORER | OFN_ALLOWMULTISELECT | OFN_FILEMUSTEXIST | OFN_NOCHANGEDIR}
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
		alert("至少加入 2 份 PDF。", true)
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
	if isMerging {
		return
	}
	out := chooseOutput()
	if out == "" {
		return
	}
	snapshot := append([]string(nil), winFiles...)
	isMerging = true
	status("正在合併 PDF，請勿關閉此視窗…")
	go func() {
		count, e := mergePDFFiles(snapshot, out)
		resultMu.Lock()
		latestResult = mergeResult{path: out, count: count, err: e}
		resultMu.Unlock()
		pPostMessage.Call(hwndMain, WM_APP+1, 0, 0)
	}()
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
	case WM_CREATE:
		traceStartup("WM_CREATE received")
		hwndMain = h
		initUI()
		traceStartup("WM_CREATE UI initialized")
		return 0
	case WM_COMMAND:
		id := int(w & 0xFFFF)
		if (id == ID_ADD || id == ID_UP || id == ID_DOWN || id == ID_REMOVE || id == ID_CLEAR || id == ID_MERGE || id == ID_ADD_ZIP) {
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
		if len(os.Args) > 1 { handleInputFiles(os.Args[1:]) }
		return 0
	case WM_APP + 1:
		traceStartup("WM_APP+1 received")
		resultMu.Lock()
		result := latestResult
		resultMu.Unlock()
		traceStartup("WM_APP+1 result obtained")
		isMerging = false
		traceStartup("WM_APP+1 flags restored")
		if result.err != nil {
			traceStartup("WM_APP+1 failure update begin")
			status("合併失敗：" + result.err.Error())
			alert("合併失敗：\n"+result.err.Error(), true)
		} else {
			traceStartup("WM_APP+1 success update begin")
			status(fmt.Sprintf("已完成 %d 頁%s：%s", result.count, result.warning, result.path))
		}
		traceStartup("WM_APP+1 done")
		return 0
	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProc.Call(h, uintptr(m), w, l)
	return r
}
func main() {
	traceStartup("main entry")
	pSetProcessDPIAware.Call()
	instance, _, _ := pGetModuleHandle.Call(0)
	className := str("FastPDFMergeNativeGUI")
	wc := wndClass{cbSize: uint32(unsafe.Sizeof(wndClass{})), lpfnWndProc: windowProc, hInstance: instance, hbrBackground: 6, lpszClassName: className}
	atom, _, e := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	traceStartup(fmt.Sprintf("RegisterClassExW atom=%d error=%v", atom, e))
	if atom == 0 {
		pMessageBox.Call(0, uintptr(unsafe.Pointer(str("無法註冊 Windows 視窗類別："+e.Error()))), uintptr(unsafe.Pointer(str("公文 PDF 快合｜啟動失敗"))), MB_OK|MB_ICONERROR)
		return
	}
	screenX, _, _ := pGetSystemMetrics.Call(0)
	screenY, _, _ := pGetSystemMetrics.Call(1)
	x := int((screenX - 660) / 2)
	y := int((screenY - 610) / 2)
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	traceStartup("CreateWindowExW begin")
	hwnd, _, windowErr := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(str("公文 PDF 快合｜極簡版"))), uintptr(WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX|WS_VISIBLE), uintptr(x), uintptr(y), 660, 578, 0, 0, instance, 0)
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
