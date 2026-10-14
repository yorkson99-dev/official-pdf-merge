# 公文 PDF 快合 v1.6.3｜Office COM 逐份串流合併

## 版本定位
本版由 v1.6.2 延伸，維持 Microsoft Word／Excel／PowerPoint COM 原生排版與 Windows 原生 GUI。專屬分支 `maintenance/v1.6.3-stream`，不更動 main 或 v1.7.0 NDC ODF 版本。

## 核心改良
- 每次僅解析一份 PDF，重定位來源 PDF 物件後**立即依序寫入唯一的合併輸出資料流**；不在記憶體保留全部附件的已解析 PDF 物件，也不反覆讀取／寫回已合併的內容。
- 先寫 Catalog 和個別文件物件，**最後才完成共同 Pages 節點、xref 和 trailer**，輸出符合法定的完整 PDF 結構。
- 僅產生一個同目錄暫存最終PDF（原子發布使用），沒有每3份／每階段的中間合併PDF。採256KiB緩衝並同步寫入，任一附件發生錯誤即刪除尚未發布的輸出。
- ZIP 主文在前、ATTCH 數字排序、機關／日期／文號命名、拒絕不支援副檔名、輸出不覆蓋等業務規則完全保留。
- DOCX、ODT、ODS 等仍交由 Microsoft Office 原生轉檔，每3份可編輯附件一個階段、嚴格循序執行；必要的Office來源與轉檔PDF仍短暫存於私人工作資料夾，轉換完成會清理。單一大型PDF仍需完整解析，所以本版不是固定記憶體用量的PDF解析器。

## 自動測試
- 12份PDF串流寫入、檢查遞延Pages節點、xref/trailer與輸出PDF頁數。
- 12份混合附件（9PDF、1DOCX、2ODT）的轉檔模擬、原始附件順序與合併PDF完整性。
- 7份Office附件依 3＋3＋1 分段；末份PDF無效時不留下未完成輸出；禁止覆寫已存在的PDF。
- Windows GUI 啟動、ZIP合併、人工排序、單一EXE直接拖入及自動命名測試。

## 限制與安全
- Windows CI沒有 Microsoft Office，所以真實 DOCX／ODT／ODS 的 Office COM 批次作業須在已安裝 Office 的合法環境中驗證。
- 這是效能與記憶體管理改進，**尚未證實 Apex One 誤判或攔截已解除**；不要為了測試而關閉或繞過端點防護。若再次被攔截，請留存同一版EXE、SHA-256和警示事件，供Trend Micro正式分析。
- PDF合併不保證保留來源數位簽章、互動表單、書籤等進階功能，正式文件仍須檢視輸出結果。
