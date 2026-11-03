# 公文 PDF 快合｜v1.6.1-office2 常見圖片格式支援

基底：v1.6.1-office2。獨立分支：feature/v1.6.1-office2-images。不執行 CI、未發布 Release。

支援 PNG、JPG、JPEG、JPE、JFIF、GIF、BMP、DIB、TIFF、TIF、ICO、JXR、WDP、HDP 的本機圖片轉 PDF；WebP、HEIC、HEIF、HIF、AVIF 依 Windows WIC 已安裝的解碼器支援。圖片輸出 A4 等比置中 PDF；TIFF 多頁輸出所有頁；動畫只取首張影格。每頁最多 4,000 萬像素，TIFF 最多 200 頁。

圖片支援公文 ZIP ATTCH、一般文件排序清單與拖檔到 EXE，並可與 Office、PDF 混合。圖片不算入超過 2 份 Office 轉檔的警告門檻；不透過 Office COM 轉換圖片。無法解碼時不默默略過。

不支援 SVG 的執行式向量內容。未變更任何端點防護設定；資安相容性仍待審查。
