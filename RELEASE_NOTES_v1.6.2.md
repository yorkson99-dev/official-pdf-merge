# 公文 PDF 快合 v1.6.2｜Office COM 多附件維護版

## 基準版本

以 v1.6.1 原始碼為基底，在 `maintenance/v1.6.2-office` 分支維護，並不以 v1.7.0 的 NDC ODF 架構為基底；不會更新 `main` 或取代 v1.7.0。

## 改進

- 保留 Microsoft Word、Excel、PowerPoint 原生 COM 文件轉檔，不需額外命令列或 PowerShell。
- 可編輯附件最多每3份作為一個階段，**依序**轉換；同一階段連續 Word 文件共用同一 Office COM 執行個體，減少重複啟動。
- 原生 PDF 仍優先直接在記憶體解析，無須解壓到硬碟。
- 多附件完成後只輸出一份最終合併 PDF，不產生每3份一個中間合併結果。
- 合併 PDF 輸出改採 256 KiB 緩衝寫入，維持完整寫入後才重新命名的原子發布流程。
- 保留主文先行、ATTCH編號排序、公文機關命名、完整日期與文號命名及不覆寫既有檔案。

## 測試

- 合成 12 份文件（9 PDF、1 DOCX、2 ODT）：模擬 Office 批次轉檔，檢查順序、12頁結果及輸出資料夾只有一份最終 PDF。
- 7 份可編輯文件分段測試：3＋3＋1，轉檔失敗不留下最終輸出。
- Windows GitHub Actions：Go 單元測試、COM IDispatch ABI 測試、GUI 啟動、公文 ZIP、手動排序、直接拖入 EXE 等流程。

**尚未驗證：** GitHub Windows Runner 沒有安裝 Office，真正 Word/Excel/PowerPoint 的連續 COM 工作階段仍須由有 Office 的電腦實測。也尚未驗證 Trend Micro Apex One 是否仍會發出警示。

## 資訊安全說明

資源與磁碟 I/O 改良不等於 Apex One 誤判已解決，也不保證端點安全防護將允許執行。遇到隔離或阻擋請停止測試，留存警示記錄和 EXE 雜湊，將固定樣本向 Trend Micro 反映疑似誤判；勿變更防毒安全政策。
