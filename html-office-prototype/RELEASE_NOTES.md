# HTML × Microsoft Office 本機橋接測試原型 v0.1.0

此版本並非正式替代公文PDF快合EXE，而是驗證「Edge HTML + 已安裝的Microsoft Office原生排版引擎」的架構。

- 不含自行編譯的EXE，不連線到雲端，不需要PowerShell、NDC ODF或後端HTTP服務。
- Windows內建 cscript.exe 執行可見的 OfficeBridge.vbs，本機資料夾交換檔案，COM呼叫Word/Excel/PowerPoint匯出PDF。
- HTML處理「來文」ZIP解析、主文與ATTCH數字排序、PDF合併及另存。原生瀏覽器操作，需要使用者選擇Bridge工作資料夾與最後儲存位置。
- 隨ZIP離線內附 JSZip 3.10.1、pdf-lib 1.17.1，運作過程不會向CDN下載函式庫。
- GitHub Windows CI通過JS單元測試、Office腳本靜態檢查及離線ZIP封裝測試。
- **尚未在有Microsoft Office且受Apex One管控的電腦驗證可用性。** 禁止巨集／腳本的機關電腦可能拒絕啟動；若攔截，應停止測試並保留事件紀錄，不能藉此規避防護政策。
- 尚未移植自動發文機關／文號解析、一般手動多檔排序、數位簽章保留；Office排版仍須實機核對。

使用前先完整解壓ZIP，開啟README.md並使用無機敏測試資料。
