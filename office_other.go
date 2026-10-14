//go:build !windows

package main

import "fmt"

func convertDocumentToPDF(input, output string) error {
    return fmt.Errorf("NDC ODF 自動轉檔僅支援 Windows：%s", input)
}
func convertDocumentsToPDFBatch(inputs, outputs []string) error {
    return fmt.Errorf("NDC ODF 批次轉檔僅支援 Windows")
}
