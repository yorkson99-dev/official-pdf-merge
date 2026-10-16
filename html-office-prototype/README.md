# 公文 PDF 快合｜HTML × Microsoft Office 本機橋接診斷版 v0.2.0

> **原型／不是正式版。** 請先用非機敏測試文件驗證。這套方案**不會編譯、散布自行製作的 EXE**；Windows仍須允許執行本機 .cmd／.vbs 腳本。無法保證 Apex One 不攔截，勿變更防護政策或使用未核准的腳本。

## 概念

```text
Microsoft Edge 開啟 index.html
    ├─ JSZip：在瀏覽器解析「來文」ZIP並依 ATTCH 編號排序
    ├─ 純 PDF：保留在瀏覽器中
    └─ DOCX/ODT/ODS/PPTX等：使用者授權的本機資料夾
          └─ OfficeBridge.vbs（以 Windows 自帶 cscript.exe 執行）
                └─ Microsoft Word / Excel / PowerPoint COM 輸出 PDF
    └─ pdf-lib：將全部 PDF 依順序合併
    └─ 使用者按「儲存合併 PDF」，選擇本機存放位置
```

沒有遠端後端、API伺服器、CDN、資料上傳，也不使用 PowerShell 或 NDC ODF。HTML無法直接調用 Windows COM；**橋接腳本是必要環節**。檔案交換方案使用 Edge 提供的 File System Access API，所以要授予**明確的工作資料夾權限**。

## 使用說明（Windows 10／11、Microsoft Edge、已安裝正版 Microsoft Office）

1. 從 GitHub Release 下載 `OfficePDFMerge-HTML-Office-v0.2.0.zip` 並完整解壓（**不可直接在ZIP內開啟HTML**）。
2. 若 ZIP 包含 Office 文件，且電腦允許依單位政策執行 .cmd／.vbs，雙擊 `啟動 Office 橋接.cmd`，保持可見的命令提示字元視窗開啟。若該操作被 Windows 或 Apex One 阻擋，**停止，不要改成其他方式規避防護**。
3. 用 Microsoft Edge 開啟 `index.html`，**直接拖入公文 ZIP 或個別PDF／DOCX／ODT／ODS等文件**；也可以按「選擇公文ZIP」或「加入個別檔案」。會自動列出主文及ATTCH附件，支援向上、向下、移除及繼續追加文件。
4. 點「選擇 OfficeBridgeData 資料夾」，只選擇套件內同名的資料夾。Edge可能要求讀寫權限。**此時網頁會進行9秒無文件連線測試：成功顯示「連線測試成功」，失敗會提示可能未啟動腳本或選錯目錄。** 純PDF無須連接。
5. 網頁會先用**離線PDF.js辨識來文機關**，若可辨識則沿用原版「○○機關公文及其附件.pdf」；勾選完整命名可使用民國日期與發文字號。掃描影像及無法抽出文字的主文會明確提示手動輸入。按「開始處理」，有Office附件時會即時顯示Word／Excel／PowerPoint的啟動、開啟與PDF匯出階段；失敗會回報**階段、文件及COM錯誤碼**。
6. 出現「合併完成」後，按「儲存合併 PDF」，自行指定檔名及目錄。輸出前請檢查所有附件完整性、頁數及版面。

## 已實作

- 保留專案公文 ZIP：`來文` 目錄／一份主文PDF／同主檔基底 ATTCH1、ATTCH2……。
- 依**數字**排序，而非字典序；排除 opinion、(before)、(after)、合併版；未知附件格式不會默默忽略。
- Word：DOC、DOCX、DOCM、ODT、RTF、TXT；Excel：XLS、XLSX、XLSM、ODS、CSV、TSV；PowerPoint：PPT、PPTX、PPTM、ODP。
- Office程式只按需要啟動，逐份開啟及輸出PDF、禁用Office自動巨集、結束後釋放COM物件。
- PDF合併在瀏覽器完成，無需自製EXE或網路服務，最後由使用者另存完整PDF。
- 輸入ZIP限750 MiB、單份限200 MiB；檢查附件重複編號及ZIP CRC。HTML只從使用者明確選取的檔案／資料夾讀取。
- 使用JSZip 3.10.1、pdf-lib 1.17.1及PDF.js 3.11.174，全部包含於離線包內，無需CDN。
- GitHub Actions 內的Node測試覆蓋：ZIP附件排序／PDF頁序／未知格式拒絕／重複編號／機關及發文字號解析／錯誤碼呈現，並由**真正Windows cscript.exe執行橋接器**，驗證它能正確回傳錯誤而不是陷入等待。

## 尚未驗證或限制

- **Microsoft Office真實端到端轉檔尚未在安裝Office的Windows實機驗證**。GitHub Windows CI雖可測試HTML與PDF/ZIP，不能憑CI成功表示本機VBScript/Word實測成功。
- 使用者需自行啟動可見的橋接視窗，並授權瀏覽器工作資料夾。腳本在公務電腦可能被端點控管政策禁用，**不保證Apex One允許**。如果9秒連線測試失敗，先查看CMD視窗是否出現VBScript錯誤，不要反覆重試或變更安全設定。
- HTML **不能**在一般Edge安全模式下無提示直接呼叫Word COM，也不能無權限自動儲存到來源ZIP相同目錄。
- 原PDF.js文字抽取已加入：**可搜尋文字的主文PDF會嘗試辨識機關、民國日期、發文字號**；特定字型、掃描件或複雜版面可能辨識失敗，應依實際公文檢查，未辨識時保留手動命名。
- 已加入拖放個別檔案、追加、清空、向上／向下及移除；但尚未提供原Win32 GUI相同的所有快捷鍵及自動存回ZIP所在資料夾功能。
- pdf-lib會重新產生PDF文件，來源數位簽章、AcroForm欄位、書籤、嵌入附件不保證保留；部分加密或非標準PDF不能直接合併。請保存原件並檢視合併結果。
- 瀏覽器處理會消耗記憶體；此版本不是v1.6.3 Go物件級串流寫入器，不能直接假定它更省RAM。
- 如橋接等待超時，未完成的工作資料夾可能保留原始檔案；在關閉橋接之後，請檢查 `OfficeBridgeData/jobs` 並依機關資料處理規則清除。
- 若 Edge 的 `showDirectoryPicker` 不可用（不支援、不允許本機file://或遭政策限制），則含Office附件模式無法工作；可用純PDF ZIP合併模式。

## 安全資訊

- 不執行ZIP內的腳本或程式：只允許指定副檔名，命名並放置至受控工作資料夾後由已安裝Office轉PDF。
- 一般Edge腳本沒有權限直接執行EXE、COM；bridge目錄授權只限本機資料夾。
- 橋接工作資料夾為讀寫可執行腳本的本機交換介面，若有其他程序可以寫入工作資料夾，就可能提交轉檔作業。不要將工作資料夾分享至網路、交給不受信任的人或存放機敏文件。
- Office是互動式桌面應用，工作時可能出現需要使用者確認的對話框；並非Microsoft支援的非互動伺服器自動化架構。
- 若機關禁止 .vbs／.cmd 或 Office COM 自動化，應由資訊管理人員評估合規替代方式，不應關閉防護或改寫程式規避限制。

## 原始碼

此原型在專用分支 `prototype/html-office-bridge`，不覆蓋 main、v1.6.2 或 v1.6.3 的正式Windows版本。

後續驗證重點：**有Office實機的完整轉檔**、PDF.js對實際公文（非影像型PDF）的抬頭辨識、Big5 ZIP相容性、HTML選擇資料夾權限，以及Apex One實際偵測事件。
