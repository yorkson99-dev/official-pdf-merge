# 公文 PDF 快合｜v1.7.0 離線 NDC ODF 轉檔版

適用 Windows 10／11（x64）的 Win32 原生、公文 ZIP 一鍵合併工具。

## 安裝需求

- 主程式為單一免安裝的 `OfficialPDFMerge.exe`，不需要 Python、Go、Java、.NET 或網路。
- **可編輯文件轉檔需要事先安裝 NDC ODF Application Tools 3.3.3（x64）**。預設查找：
  `C:\Program Files\NDC ODF Application Tools\program\soffice.com`
- 本版**不使用 Microsoft Office COM、不啟動 PowerShell**，不再包含原有 COM 轉檔模組。
- 公文與附件一律在本機處理，不上傳雲端，也不啟動本機 HTTP 服務。
- 純 PDF 合併不需要 NDC ODF，也不會啟動轉檔程式。

## 公文 ZIP 自動合併

將公文 ZIP 拖入已開啟的程式，或拖到 EXE 圖示上：

1. 解析 `來文` 資料夾主文 PDF，依 `ATTCH1`、`ATTCH2` 等數字排序附件。
2. 排除 opinion、(after)、(before)、合併版及稿件；遇到不支援的附件格式則報錯，不會默默省略附件。
3. 純 PDF 附件採記憶體解析合併（預設 ZIP 內 PDF 合計不超過 96 MiB 時）。大型 PDF 才使用必要的暫存。
4. 將 DOC／DOCX／DOCM／ODT／RTF／TXT、XLS／XLSX／XLSM／ODS／CSV／TSV、PPT／PPTX／PPTM／ODP 附件，以**一次 NDC ODF 背景批次轉檔**為 PDF，再依原 ATTCH 順序合併。
5. 完成後自動存回 ZIP 所在目錄，檔名為「機關公文.pdf」或「機關公文及其附件.pdf」。可勾選完整日期與文號命名；檔名衝突時自動編號，不覆寫原檔。

轉換失敗、找不到轉檔程式、缺漏 PDF 或逾時時，停止合併並顯示錯誤，不會輸出刻意略過附件的「完成」檔案。

## 一般文件合併

在程式內加入支援的 PDF 或可編輯文件，可調整順序、移除、清空，預設將「合併檔案.pdf」存至第一份文件所在資料夾。同名會加編號。勾選「合併時開啟另存視窗」時可自行選擇檔名和位置。

也可將一組檔案直接拖入 EXE 圖示以自動產生合併檔；單獨一份可編輯文件直接拖入 EXE 時，會輸出相同基本檔名的 PDF。純 PDF 合併不建立不必要的 Office 工作資料夾。

## NDC ODF 轉檔穩定性與隔離

- 固定查找已安裝的 `soffice.com`，執行 `--headless --convert-to pdf`，**不是透過 shell／PowerShell**。
- 每次批次轉檔建立專用的 `UserInstallation` 設定檔，避免與使用者已開啟的 NDC ODF 共用設定檔；轉檔完成後清理本次暫存。這有助於降低設定檔鎖定衝突，但尚需在 NDC ODF 3.3.3 實機驗證。
- 同一批附件僅啟動一次 NDC ODF，使用來源檔清單與輸出目標的對應表恢復原始排序。來源檔名相同時會另行暫存為不重複名稱。
- 轉檔逾時設定為 2 分鐘＋每份附件 1 分鐘，最多 12 分鐘；若逾時或檔案缺漏，會清楚報錯。
- 外部 PDF 輸出先檢查格式標頭，再交由現有 PDF 解析器確認頁數及合併能力。原始 ZIP 與來源文件不變動。
- LibreOffice 系列轉檔引擎對 DOCX、XLSX、PPTX 的輸出格式可能與 Microsoft Office 有排版差異；尤其 Excel/Calc 列印範圍、CSV 編碼、中文字型、巨集文件與外部連結必須自行核對。

## 版本驗證範圍

GitHub Windows Actions 已納入單元測試、NDC ODF 指令及獨立設定檔 URI 測試、模擬 NDC 批次轉換（包括同名文件與缺漏轉檔結果）、ZIP 附件順序、PDF 合併位元組一致性、原生 GUI 啟動及手動清單操作測試。

**GitHub Windows 執行環境不含公務電腦上的 NDC ODF**。真實的 NDC ODF 3.3.3 轉檔、ODT／DOCX／XLSX／PPTX 排版，以及 `bootstrap.ini` 警告是否改善，仍須於已安裝的電腦實測。

你已透過 CMD 實測 NDC ODF 3.3.3 可將 TXT、ODT 匯出為 PDF，但尚未實測新版 EXE 與 NDC 的聯動。

## 程式碼及資安

- 以 Go 標準函式庫、Win32 API 開發；不需要下載執行階段或套件。
- PDF 合併會先寫入輸出目錄內的暫存結果，再於成功時正式命名，以免留下部分輸出的公文 PDF。
- `OfficialPDFMerge.exe` 尚未具備受信任的程式碼簽章。Trend Micro Apex One 仍可能直接以機器學習或行為監控攔截 EXE；**改用 NDC ODF 不能保證解除偵測，也不會改動 Apex One 的設定**。
- 如發生偵測，保留警示類型、時間與 SHA-256，使用固定版本向 Trend Micro 查證是否為誤判。不要停用或規避防毒監控。
- PDF 合併不保證保留數位簽章效力、書籤、互動表單及 PDF 附件，正式公文請核對結果。
- 測試文件全為合成資料，不在 GitHub 存放真實公文。

## 開發建置（一般使用者無須操作）

```powershell
go test ./...
$env:GOOS="windows"
$env:GOARCH="amd64"
$env:CGO_ENABLED="0"
go build -trimpath -ldflags="-H=windowsgui -s -w" -o "公文PDF快合.exe" .
```
