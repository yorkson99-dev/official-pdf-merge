//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	pShowWindow               = u32.NewProc("ShowWindow")
	pDestroyWindow            = u32.NewProc("DestroyWindow")
	pPostQuitMessage          = u32.NewProc("PostQuitMessage")
	pSetWindowLongPtr         = u32.NewProc("SetWindowLongPtrW")
	pCallWindowProc           = u32.NewProc("CallWindowProcW")
	pFindWindow               = u32.NewProc("FindWindowW")
	pFindWindowEx             = u32.NewProc("FindWindowExW")
	pEnumWindows              = u32.NewProc("EnumWindows")
	pGetWindowThreadProcessID = u32.NewProc("GetWindowThreadProcessId")
	pSendMessageTimeout       = u32.NewProc("SendMessageTimeoutW")
	pGetSystemMetrics         = u32.NewProc("GetSystemMetrics")
	pGetModuleHandle          = k32.NewProc("GetModuleHandleW")
	pOpenProcess              = k32.NewProc("OpenProcess")
	pCloseHandle              = k32.NewProc("CloseHandle")
	pVirtualAllocEx           = k32.NewProc("VirtualAllocEx")
	pVirtualFreeEx            = k32.NewProc("VirtualFreeEx")
	pWriteProcessMemory       = k32.NewProc("WriteProcessMemory")
	pReadProcessMemory        = k32.NewProc("ReadProcessMemory")
	pSHGetFolderPath          = s32.NewProc("SHGetFolderPathW")
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
	SW_HIDE              = 0
	SW_SHOW              = 5
	BN_CLICKED           = 0
	LB_ADDSTRING         = 0x0180
	LB_RESETCONTENT      = 0x0184
	LB_SETCURSEL         = 0x0186
	LB_GETCURSEL         = 0x0188
	LB_ITEMFROMPOINT     = 0x01A9
	LVM_FIRST            = 0x1000
	LVM_GETITEMCOUNT     = LVM_FIRST + 4
	LVM_GETITEMPOSITION  = LVM_FIRST + 16
	LVM_GETITEMTEXTW     = LVM_FIRST + 115
	SMTO_ABORTIFHUNG     = 0x2
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
	PROCESS_VM_OPERATION = 0x0008
	PROCESS_VM_READ      = 0x0010
	PROCESS_VM_WRITE     = 0x0020
	MEM_COMMIT           = 0x1000
	MEM_RESERVE          = 0x2000
	MEM_RELEASE          = 0x8000
	PAGE_READWRITE       = 0x04
	VK_DELETE            = 0x2E
	VK_CONTROL           = 0x11
	VK_UP                = 0x26
	VK_DOWN              = 0x28
	ID_ADD               = 101
	ID_SORT_COL          = 102
	ID_SORT_ROW          = 103
	ID_UP                = 104
	ID_DOWN              = 105
	ID_REMOVE            = 106
	ID_CLEAR             = 107
	ID_MERGE             = 108
	ID_ADD_ZIP           = 109
	ID_FULL_NAME         = 110
	ID_TAB_ZIP           = 111
	ID_TAB_PDF           = 112
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
type iconPos struct{ x, y int32 }
type mergeResult struct {
	path  string
	count int
	err   error
}
type desktopResult struct {
	files []string
	original []string
	ok bool
	horizontal bool
}

var (
	hwndMain, hwndList, hwndStatus, hwndMerge, hwndZip, hwndDetail uintptr
	hwndTabZip, hwndTabPDF uintptr
	zipControls, pdfControls []uintptr
	uiMode int
	isSorting bool
	latestDesktop desktopResult
	oldListProc                                                    uintptr
	winFiles                                                       []string
	dragFrom                                                       = -1
	isMerging                                                      bool
	resultMu                                                       sync.Mutex
	latestResult                                                   mergeResult
	fontHandle                                                     uintptr
	windowProc                                                     = syscall.NewCallback(wndProc)
	listProc                                                       = syscall.NewCallback(listWndProc)
	enumProc                                                       = syscall.NewCallback(enumWindowForDesktop)
	desktopList                                                    uintptr
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
func addPanelControl(mode int, class, title string, x, y, w, h int, style uint32, id int) uintptr {
	p := makeChild(class, title, x, y, w, h, style, id)
	if mode == 0 { zipControls = append(zipControls, p) } else { pdfControls = append(pdfControls, p) }
	return p
}
func setMode(mode int) {
	uiMode = mode
	for _, h := range zipControls {
		if mode == 0 { pShowWindow.Call(h, SW_SHOW) } else { pShowWindow.Call(h, SW_HIDE) }
	}
	for _, h := range pdfControls {
		if mode == 1 { pShowWindow.Call(h, SW_SHOW) } else { pShowWindow.Call(h, SW_HIDE) }
	}
	if mode == 0 {
		pSetWindowText.Call(hwndTabZip, uintptr(unsafe.Pointer(str("● 公文 ZIP｜自動合併"))))
		pSetWindowText.Call(hwndTabPDF, uintptr(unsafe.Pointer(str("一般 PDF｜手動排序"))))
		if !isMerging { status("待命：拖入公文 ZIP，即自動存回 ZIP 同一資料夾。") }
	} else {
		pSetWindowText.Call(hwndTabZip, uintptr(unsafe.Pointer(str("公文 ZIP｜自動合併"))))
		pSetWindowText.Call(hwndTabPDF, uintptr(unsafe.Pointer(str("● 一般 PDF｜手動排序"))))
		if !isMerging { refresh(selected()) }
	}
}
func initUI() {
	fontHeight := int32(-18)
	fontHandle, _, _ = pCreateFont.Call(uintptr(fontHeight), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(str("Microsoft JhengHei UI"))))
	hwndTabZip = makeChild("BUTTON", "● 公文 ZIP｜自動合併", 22, 18, 342, 45, WS_TABSTOP, ID_TAB_ZIP)
	hwndTabPDF = makeChild("BUTTON", "一般 PDF｜手動排序", 374, 18, 361, 45, WS_TABSTOP, ID_TAB_PDF)

	addPanelControl(0, "STATIC", "公文 ZIP：拖入視窗，即開始自動合併", 35, 113, 675, 33, 0, 0)
	addPanelControl(0, "STATIC", "處理方式：只取來文主文及 ATTCH1、ATTCH2……，依序合併。", 35, 166, 688, 32, 0, 0)
	addPanelControl(0, "STATIC", "自動排除 opinion、合併版及稿件；原 ZIP 不變動。", 35, 204, 690, 32, 0, 0)
	addPanelControl(0, "STATIC", "輸出位置：ZIP 所在資料夾。名稱重複時自動編號，不覆寫。", 35, 242, 698, 32, 0, 0)
	hwndDetail = addPanelControl(0, "BUTTON", "以完整日期、文號命名（未勾選為「機關公文」）", 35, 308, 665, 35, WS_TABSTOP|0x00000003, ID_FULL_NAME)
	hwndZip = addPanelControl(0, "BUTTON", "選擇 ZIP…（也可直接拖入）", 35, 386, 665, 52, WS_TABSTOP, ID_ADD_ZIP)
	addPanelControl(0, "STATIC", "辨識不完整時，以「ZIP 檔名_來文資料待確認.pdf」輸出，避免猜測。", 35, 466, 695, 40, 0, 0)

	addPanelControl(1, "STATIC", "PDF 依下方清單排序。拖曳項目或按上移／下移調整。", 22, 78, 718, 29, 0, 0)
	hwndList = addPanelControl(1, "LISTBOX", "", 22, 110, 535, 325, WS_BORDER|WS_VSCROLL|WS_TABSTOP|LBS_NOTIFY|LBS_NOINTEGRALHEIGHT, ID_LIST)
	oldListProc, _, _ = pSetWindowLongPtr.Call(hwndList, GWLP_WNDPROC, listProc)
	addPanelControl(1, "BUTTON", "加入 PDF…", 575, 110, 159, 40, WS_TABSTOP, ID_ADD)
	addPanelControl(1, "BUTTON", "↑ 上移", 575, 160, 159, 35, WS_TABSTOP, ID_UP)
	addPanelControl(1, "BUTTON", "↓ 下移", 575, 202, 159, 35, WS_TABSTOP, ID_DOWN)
	addPanelControl(1, "BUTTON", "移除選取", 575, 251, 159, 35, WS_TABSTOP, ID_REMOVE)
	addPanelControl(1, "BUTTON", "清空清單", 575, 293, 159, 35, WS_TABSTOP, ID_CLEAR)
	addPanelControl(1, "BUTTON", "桌面直排", 575, 344, 159, 35, WS_TABSTOP, ID_SORT_COL)
	addPanelControl(1, "BUTTON", "桌面橫排", 575, 387, 159, 35, WS_TABSTOP, ID_SORT_ROW)
	hwndMerge = addPanelControl(1, "BUTTON", "合併 PDF…", 22, 462, 712, 55, WS_TABSTOP, ID_MERGE)
	hwndStatus = makeChild("STATIC", "", 22, 551, 713, 49, 0, ID_STATUS)
	pDragAcceptFiles.Call(hwndMain, 1)
	pDragAcceptFiles.Call(hwndList, 1)
	setMode(0)
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
	status(fmt.Sprintf("已加入 %d 份 PDF。未自動掃描桌面圖示；請確認清單順序。", count))
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
func desktopFolder() string {
	var buf [32768]uint16
	result, _, _ := pSHGetFolderPath.Call(0, 0x10, 0, 0, uintptr(unsafe.Pointer(&buf[0])))
	if int32(result) != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:])
}
func enumWindowForDesktop(h uintptr, l uintptr) uintptr {
	def, _, _ := pFindWindowEx.Call(h, 0, uintptr(unsafe.Pointer(str("SHELLDLL_DefView"))), 0)
	if def != 0 {
		list, _, _ := pFindWindowEx.Call(def, 0, uintptr(unsafe.Pointer(str("SysListView32"))), 0)
		if list != 0 {
			desktopList = list
			return 0
		}
	}
	return 1
}
func desktopIcons() map[string]iconPos {
	desktopList = 0
	pEnumWindows.Call(enumProc, 0)
	if desktopList == 0 {
		return nil
	}
	var pid uint32
	pGetWindowThreadProcessID.Call(desktopList, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return nil
	}
	proc, _, _ := pOpenProcess.Call(PROCESS_VM_OPERATION|PROCESS_VM_READ|PROCESS_VM_WRITE, 0, uintptr(pid))
	if proc == 0 {
		return nil
	}
	defer pCloseHandle.Call(proc)
	remote, _, _ := pVirtualAllocEx.Call(proc, 0, 4096, MEM_COMMIT|MEM_RESERVE, PAGE_READWRITE)
	if remote == 0 {
		return nil
	}
	defer pVirtualFreeEx.Call(proc, remote, 0, MEM_RELEASE)
	count := int(send(desktopList, LVM_GETITEMCOUNT, 0, 0))
	if count < 1 || count > 20000 {
		return nil
	}
	result := make(map[string]iconPos, count)
	for i := 0; i < count; i++ {
		var posBytes [8]byte
		if !remoteMessage(desktopList, LVM_GETITEMPOSITION, uintptr(i), remote) {
			continue
		}
		var readBytes uintptr
		r, _, _ := pReadProcessMemory.Call(proc, remote, uintptr(unsafe.Pointer(&posBytes[0])), 8, uintptr(unsafe.Pointer(&readBytes)))
		if r == 0 || readBytes != 8 {
			continue
		}
		pos := iconPos{int32(binary.LittleEndian.Uint32(posBytes[:4])), int32(binary.LittleEndian.Uint32(posBytes[4:8]))}
		// 64-bit LVITEMW: pszText at offset 24, cchTextMax at 32.
		var item [96]byte
		binary.LittleEndian.PutUint32(item[0:4], 1) // LVIF_TEXT
		binary.LittleEndian.PutUint32(item[4:8], uint32(i))
		binary.LittleEndian.PutUint64(item[24:32], uint64(remote+256))
		binary.LittleEndian.PutUint32(item[32:36], 500)
		r, _, _ = pWriteProcessMemory.Call(proc, remote, uintptr(unsafe.Pointer(&item[0])), uintptr(len(item)), 0)
		if r == 0 || !remoteMessage(desktopList, LVM_GETITEMTEXTW, uintptr(i), remote) {
			continue
		}
		buf := make([]uint16, 500)
		r, _, _ = pReadProcessMemory.Call(proc, remote+256, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2), 0)
		if r == 0 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(syscall.UTF16ToString(buf)))
		if label != "" {
			result[label] = pos
		}
	}
	return result
}
func remoteMessage(h uintptr, id uint32, w, l uintptr) bool {
	var ret uintptr
	result, _, _ := pSendMessageTimeout.Call(h, uintptr(id), w, l, SMTO_ABORTIFHUNG, 300, uintptr(unsafe.Pointer(&ret)))
	return result != 0
}
// Desktop positions may require cross-process Explorer calls; they MUST never
// run on the Win32 UI thread. Edits made while sorting invalidate the result.
func desktopSortedFiles(files []string, horizontal bool) ([]string, bool) {
	if len(files) < 2 { return nil, false }
	dir := desktopFolder()
	if dir == "" { return nil, false }
	positions := desktopIcons()
	if len(positions) == 0 { return nil, false }
	p := make([]iconPos, len(files))
	for i, f := range files {
		if !strings.EqualFold(filepath.Clean(filepath.Dir(f)), filepath.Clean(dir)) { return nil, false }
		name := strings.ToLower(filepath.Base(f))
		v, ok := positions[name]
		if !ok { v, ok = positions[strings.TrimSuffix(name, filepath.Ext(name))] }
		if !ok { return nil, false }
		p[i] = v
	}
	ids := make([]int, len(files))
	for i := range ids { ids[i] = i }
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := p[ids[i]], p[ids[j]]
		if horizontal {
			if a.y/52 != b.y/52 { return a.y < b.y }
			return a.x < b.x
		}
		if a.x/52 != b.x/52 { return a.x < b.x }
		return a.y < b.y
	})
	out := make([]string, len(ids))
	for i, id := range ids { out[i] = files[id] }
	return out, true
}
func startDesktopSort(horizontal bool) {
	if isSorting { return }
	if len(winFiles) < 2 { status("請先加入至少 2 份 PDF。"); return }
	original := append([]string(nil), winFiles...)
	isSorting = true
	status("背景讀取桌面圖示排列位置中，視窗仍可操作…")
	go func() {
		result := desktopResult{original: original, horizontal: horizontal}
		defer func() {
			if recover() != nil { result.ok = false }
			resultMu.Lock()
			latestDesktop = result
			resultMu.Unlock()
			pPostMessage.Call(hwndMain, WM_APP+2, 0, 0)
		}()
		result.files, result.ok = desktopSortedFiles(original, horizontal)
	}()
}

// ZIP exports use a separate workflow rather than adding opinion sheets to the manual list.
func handleInputFiles(files []string) {
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
		setMode(0)
		if len(zips) != 1 || len(pdfs) > 0 {
			alert("請一次拖入一份公文 ZIP；一般 PDF 請另外拖入。", true)
			return
		}
		startOfficialZIP(zips[0])
		return
	}
	if len(pdfs) > 0 { setMode(1); appendFiles(pdfs) }
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
func chooseNamedOutput(folder, name string) string {
	if name == "" {
		name = "請手動命名.pdf"
	}
	suggested := filepath.Join(folder, name)
	if _, e := os.Stat(suggested); e == nil {
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		for i := 2; i < 10000; i++ {
			candidate := fmt.Sprintf("%s (%d).pdf", stem, i)
			if _, e := os.Stat(filepath.Join(folder, candidate)); os.IsNotExist(e) {
				name = candidate
				break
			}
		}
	}
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(name)))
	filter := fileFilter()
	f := openFileName{lStructSize: uint32(unsafe.Sizeof(openFileName{})), hwndOwner: hwndMain,
		lpstrFilter: &filter[0], lpstrFile: &buf[0], nMaxFile: uint32(len(buf)),
		lpstrTitle: str("確認或修改公文合併檔名"), lpstrInitialDir: str(folder), lpstrDefExt: str("pdf"),
		flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_NOCHANGEDIR}
	ok, _, _ := pGetSaveFileName.Call(uintptr(unsafe.Pointer(&f)))
	if ok == 0 {
		return ""
	}
	out := syscall.UTF16ToString(buf)
	if !strings.EqualFold(filepath.Ext(out), ".pdf") {
		out += ".pdf"
	}
	if _, e := os.Stat(out); e == nil {
		alert("此檔案已存在，程式不會覆寫，請改用其他名稱。", true)
		return ""
	}
	return out
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
	setMode(0)
	detailed := send(hwndDetail, BM_GETCHECK, 0, 0) != 0
	isMerging = true
	pEnableWindow.Call(hwndZip, 0)
	pEnableWindow.Call(hwndMerge, 0)
	status("正在處理：" + filepath.Base(path))
	go func() {
		var result mergeResult
		defer func() {
			if v := recover(); v != nil { result.err = fmt.Errorf("公文 ZIP 處理異常：%v", v) }
			resultMu.Lock()
			latestResult = result
			resultMu.Unlock()
			pPostMessage.Call(hwndMain, WM_APP+1, 0, 0)
		}()
		job, err := analyzeZIP(path)
		if err != nil { result.err = err; return }
		name, nameErr := job.outputName(detailed)
		if nameErr != nil {
			fallback := cleanComponent(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
			if fallback == "" { fallback = "來文" }
			name = fallback + "_來文資料待確認.pdf"
		}
		out, err := autoOutputPath(path, name)
		if err != nil { result.err = err; return }
		result.path = out
		result.count, result.err = mergeOfficialZIP(job, out)
		if result.err == nil && nameErr != nil { result.err = fmt.Errorf("已合併成功，惟無法辨識完整檔名；請核對 %s（%v）", filepath.Base(out), nameErr) }
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
	pEnableWindow.Call(hwndMerge, 0)
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
	case ID_SORT_COL:
		startDesktopSort(false)
	case ID_SORT_ROW:
		startDesktopSort(true)
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
	case ID_TAB_ZIP:
		setMode(0)
	case ID_TAB_PDF:
		setMode(1)
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
		if (id >= ID_ADD && id <= ID_ADD_ZIP) || id == ID_TAB_ZIP || id == ID_TAB_PDF {
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
	case WM_APP + 2:
		resultMu.Lock()
		r := latestDesktop
		resultMu.Unlock()
		isSorting = false
		if !r.ok { status("無法取得完整桌面排序，可在清單中拖曳或使用上移／下移。"); return 0 }
		if len(r.original) != len(winFiles) { status("清單已變更，請重新排序。"); return 0 }
		for i := range r.original {
			if r.original[i] != winFiles[i] { status("清單已變更，請重新排序。"); return 0 }
		}
		winFiles = r.files
		refresh(0)
		if r.horizontal { status("已依桌面橫排排序。") } else { status("已依桌面直排排序。") }
		return 0
	case WM_APP + 1:
		resultMu.Lock()
		result := latestResult
		resultMu.Unlock()
		isMerging = false
		pEnableWindow.Call(hwndMerge, 1)
		pEnableWindow.Call(hwndZip, 1)
		if result.err != nil {
			status("合併失敗：" + result.err.Error())
			alert("合併失敗：\n"+result.err.Error(), true)
		} else {
			status(fmt.Sprintf("已完成 %d 頁：%s", result.count, result.path))
		}
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
	x := int((screenX - 770) / 2)
	y := int((screenY - 650) / 2)
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	traceStartup("CreateWindowExW begin")
	hwnd, _, windowErr := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(str("快合 PDF｜極簡快速合併"))), uintptr(WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX|WS_VISIBLE), uintptr(x), uintptr(y), 770, 630, 0, 0, instance, 0)
	traceStartup(fmt.Sprintf("CreateWindowExW hwnd=%d error=%v", hwnd, windowErr))
	if hwnd == 0 {
		pMessageBox.Call(0, uintptr(unsafe.Pointer(str("無法建立程式視窗："+windowErr.Error()))), uintptr(unsafe.Pointer(str("公文 PDF 快合｜啟動失敗"))), MB_OK|MB_ICONERROR)
		return
	}
	if len(os.Args) > 1 {
		handleInputFiles(os.Args[1:])
	}
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
