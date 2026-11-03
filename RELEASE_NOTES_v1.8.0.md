# 公文 PDF 快合 v1.8.0｜常見圖片格式轉 PDF 與混合合併

本版以 **v1.6.1-office2（需轉檔超過 2 份警告版）** 為基底，保留 Microsoft Office COM、公文 ZIP 主文／ATTCH 附件排序、進度條、現代化警告及雙欄排序清單，擴充常見圖片轉 PDF。

## 新功能

- **圖片轉 PDF**：PNG、JPG、JPEG、JPE、JFIF、GIF、BMP、DIB、TIF、TIFF、ICO、JXR、WDP、HDP。
- **依系統解碼器提供支援**：WebP、HEIC、HEIF、HIF、AVIF；必須已安裝對應 Windows WIC 解碼器，程式不會自行安裝。
- **合併模式**：可單張圖片轉 PDF，亦可將圖片、PDF 與 Office／ODF 文件混合排序合併；公文 ZIP 圖片附件仍依 ATTCH 編號排序。
- **版面**：A4 紙張、直式／橫式自動選擇、等比例縮放置中、透明處填白。多頁 TIFF 保留多頁，GIF／動畫 WebP 僅取第一影格。
- **警告門檻**：圖片及 PDF 不計入「需要 Office 轉檔超過 2 份」的慎用提醒。
- **限制**：每張最多 4,000 萬像素，多頁 TIFF 最多 200 頁；不支援 SVG。
- **離線使用**：沒有外部影像轉檔程序、沒有新增網路傳輸或安裝需求；不覆寫來源文件及既有 PDF。

## 驗收與建置

使用者於 2026 年 11 月 3 日確認測試版可正常輸出 PDF。這**不代表所有圖片格式、混合檔案情境及受管制電腦的端點防護相容性都已逐項通過驗證**。

本 Release 直接使用 2026 年 11 月 3 日建置成功的 EXE（GitHub Actions run #41573499212），僅將 EXE 的 Windows VERSIONINFO 版本號更新為 **1.8.0.0**。本次不重新編譯、不執行單元／整合 CI 測試；僅驗證版本資源、計算 SHA-256、上傳正式資產。

EXE 不具 Authenticode 數位簽章。遇 Apex One 或其他防護攔截時，須由機關資訊人員依正式程序檢查，不得自行規避防護。

GitHub 程式碼來源：`feature/v1.6.1-office2-images`；本次標籤：`v1.8.0`。原 `v1.6.1-office2` 與 `v1.7.0` 的 Release 不覆寫。
