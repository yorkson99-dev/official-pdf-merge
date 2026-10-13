# 公文 PDF 快合｜Win32 原生免安裝工具

針對 Windows 10／11（x64）公務電腦設計的獨立 PDF 合併工具。

## 系統需求與執行方式

- **原生 Win32 視窗**（傳統 Windows 按鈕、清單與開啟／儲存對話框）。
- 產生單一約 2.4 MB 的 `公文PDF快合.exe`，**執行時不需要安裝 Go、Python、Java、.NET、套件或瀏覽器執行環境**。
- PDF 和 ZIP 全程本機離線處理，不上傳公文內容。
- EXE 未經程式碼簽章。機關電腦可能因應用程式控管政策禁止執行；須由資訊管理單位依法規與機關程序審核，請勿繞過管制。
- 開發／重新建置在 GitHub Actions 上進行；**公務電腦只下載已建置的 EXE**。

## 快速使用（v1.4.0）

### 公文 ZIP（一鍵自動化）

直接把公文 ZIP 拖進程式：分析 `來文` 主文、按照 `ATTCH1`、`ATTCH2` …… 數字順序排列附件，排除意見單、合併版及稿件。**不再跳出存檔對話框**，合併 PDF 自動存回 ZIP 同一資料夾，不覆寫原檔。

- **直接合併**：PDF。
- **可編輯附件轉 PDF**：Word `DOC/DOCX/DOCM/ODT/RTF/TXT`、Excel `XLS/XLSX/XLSM/ODS/CSV/TSV`、PowerPoint `PPT/PPTX/PPTM/ODP`。
- **轉檔引擎**：預設一律優先由已安裝的 Microsoft Office 執行轉檔，包括 ODT、ODS、ODP。因部分 NDC ODF 版本在背景啟動時會顯示 `bootstrap.ini` 已鎖定等警告，程式不再自動啟動 NDC ODF。僅供排障者明確設定 `FASTPDF_ENABLE_NDC_FALLBACK=1` 時，在 Office 轉檔失敗後嘗試 NDC ODF。
- **無法轉換即中止**：不會略過未知附件、副檔名或轉檔失敗的附件而假裝整份公文已完成。
- **命名**：預設「機關公文.pdf」或「機關公文及其附件.pdf」；亦可選擇完整日期與文號。無法辨識機關時使用「ZIP檔名_來文資料待確認.pdf」。
- **轉檔效能**：純 PDF 公文不會啟動 Office；混合附件會等候本機 Word／Excel／PowerPoint 或 ODF 工具轉檔。首次啟動 Office 通常比純 PDF 慢。
- **安全性**：轉檔設為唯讀，Office 巨集自動執行已停用；文件內容不會上傳網路。請注意 Office／ODF 格式轉換仍可能有排版差異，正式公文應核對轉出結果。

### 一般文件合併（PDF、Office、ODF）

**程式已開啟時**：加入或拖入 PDF、DOC、DOCX、ODT、XLSX、ODS、PPTX、ODP 等支援格式，在清單中人工拖曳、上移／下移排列，按「合併一般文件」，才會顯示儲存位置與檔名選擇視窗。可編輯文件先轉成暫存 PDF，再依清單順序合併，不改動原檔。

**程式未開啟時**：直接選取多份文件並拖到 `OfficialPDFMerge.exe` 圖示，程式自動合併，**不顯示儲存對話框**。產生的 `合併檔案.pdf` 存於**第一份接收到的來源文件**所在資料夾，如已有相同檔名則依序產生 `合併檔案 (2).pdf` 等。不保證 Windows 傳遞多檔的順序等於桌面圖示順序；若順序有要求，應改用程式內的人工排序清單。

若單獨拖入一份可編輯文件到 EXE，則轉換為同目錄、相同基本檔名的 PDF；單獨一份 PDF 不執行合併。為測試人工清單，可使用 `--manual` 啟動參數搭配文件路徑。

### 完成狀態

底部狀態欄固定單行、截斷過長的文字，成功時只顯示檔案名稱，不直接塞入完整路徑，避免文字擠壓重疊。

### 圖示與視覺介面

原生 Win32 極簡風格，淺色卡片與重點藍色操作區；應用程式圖示已嵌入單一 EXE。啟動時不需要瀏覽器核心、網路連線或新增使用者端套件。

### 驗證範圍

GitHub CI 驗證 PDF 合併核心、模擬 ODT/DOCX/PPTX 附件篩選排序與合併、ZIP 一鍵模式及 Windows 按鈕回應。**GitHub Windows Runner 未必安裝 Office 或 NDC ODF**，因此實際 DOCX/ODT 的 Office 轉 PDF 行為仍須在已安裝軟體的公務電腦上驗證。

## 編譯

採 **純 Go 標準函式庫 + Win32 API**（呼叫 Windows 系統內建 DLL），沒有外部依賴。

```powershell
go test ./...
$env:GOOS="windows"
$env:GOARCH="amd64"
$env:CGO_ENABLED="0"
go build -trimpath -ldflags="-H=windowsgui -s -w" -o "公文PDF快合.exe" .
```

編譯作業由 GitHub Actions 執行，終端使用者不用在電腦上執行這些命令。

## 注意

- PDF 合併不保證保留數位簽章效力、互動表單、書籤與 PDF 內嵌附件，請保留來源檔，重要公務文件須核對輸出。
- 自動命名依可擷取文字層解析；影像式 PDF 或特殊編碼會使用「來文資料待確認」的檔名，請使用者核對。
- PDF 合併器偏重效率而非涵蓋所有 PDF 格式；若遇到無法支援的輸入，會停止合併並回報錯誤。
- 專案中的測試 PDF 均為合成資料，不包含實際公文內容。
- GitHub Windows Runner 驗證 EXE 啟動、公文 ZIP 自動輸出、混合格式合併核心（使用模擬轉檔器）、人工清單與直接拖檔到 EXE 的自動輸出。Office／ODF 轉換仍須於實際有安裝 Office 的公務電腦驗證。

## 來源檔案與授權

Windows 入口為 `main_windows.go`，主文解析為 `lettertext.go`，公文 ZIP 規則為 `zipmerge.go`，PDF 合併核心為 `pdfmerge.go`。

程式僅供依機關資訊安全政策核可後使用。