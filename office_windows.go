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
    kind,err:=officeKind(ext)
    if err!=nil{return err}
    return convertOfficeCOM(input, output, kind)
}

func officeKind(ext string) (string,error) {
    switch strings.ToLower(ext) {
    case ".doc",".docx",".docm",".odt",".rtf",".txt":
        return "word",nil
    case ".xls",".xlsx",".xlsm",".ods",".csv",".tsv":
        return "excel",nil
    case ".ppt",".pptx",".pptm",".odp":
        return "powerpoint",nil
    default:
        return "",fmt.Errorf("尚未支援此格式的 Office 轉換：%s",ext)
    }
}

func convertDocumentsToPDFBatch(inputs,outputs []string) error {
    return convertOfficeCOMBatch(inputs,outputs)
}
