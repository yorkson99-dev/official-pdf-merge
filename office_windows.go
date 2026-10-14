//go:build windows

package main

// Convert editable documents using the preinstalled NDC ODF Application Tools.
// Microsoft Office COM is no longer used.
func convertDocumentToPDF(input, output string) error {
    return convertDocumentsToPDFBatch([]string{input},[]string{output})
}
