//go:build windows

package main

import (
    "fmt"
    "path/filepath"
    "strings"
)

// COM replaces the previous PowerShell and NDC command-line paths entirely.
// No child shell is launched. Only installed Microsoft Office is required.
func convertDocumentToPDF(input, output string) error {
    ext:=strings.ToLower(filepath.Ext(input))
    if !supportedOfficialExt(ext) || ext==".pdf" {
        return fmt.Errorf("不支援的可編輯文件：%s",ext)
    }
    var kind string
    switch ext {
    case ".doc",".docx",".docm",".odt",".rtf",".txt":
        kind="word"
    case ".xls",".xlsx",".xlsm",".ods",".csv",".tsv":
        kind="excel"
    case ".ppt",".pptx",".pptm",".odp":
        kind="powerpoint"
    default:
        return fmt.Errorf("尚未支援此格式的 Office 轉換：%s",ext)
    }
    return convertOfficeCOM(input, output, kind)
}
