//go:build !windows

package main

import "fmt"

func convertDocumentToPDF(input, output string) error {
    return fmt.Errorf("僅支援在已安裝 Microsoft Office 的 Windows 電腦轉檔：%s", input)
}
func convertDocumentsToPDFBatch(inputs, outputs []string) error {
    return fmt.Errorf("僅支援在已安裝 Microsoft Office 的 Windows 電腦批次轉檔")
}
