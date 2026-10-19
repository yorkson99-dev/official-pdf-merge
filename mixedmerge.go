package main

import (
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
)

// mergeMixedFiles handles the PDF-only fast path as well as supported editable
// documents without installing a PDF renderer or office suite on the client.
// Inputs are processed strictly in the order provided by the caller.
func mergeMixedFiles(inputs []string,output string) (int,error) {
    return mergeMixedFilesWithProgress(inputs,output,nil)
}
func mergeMixedFilesWithProgress(inputs []string,output string,report func(int,string)) (int,error) {
    if len(inputs) == 0 { return 0, errors.New("沒有要處理的文件") }
    // A pure-PDF manual merge needs no workspace at all. Only Office files
    // require temporary converted documents.
    tempDir := ""
    defer func() {
        if tempDir != "" { os.RemoveAll(tempDir) }
    }()
    pdfs := make([]string, 0, len(inputs))
    for i, input := range inputs {
        ext := strings.ToLower(filepath.Ext(input))
        if !supportedOfficialExt(ext) {
            return 0, fmt.Errorf("不支援的文件：%s", filepath.Base(input))
        }
        if ext == ".pdf" {
            pdfs = append(pdfs, input)
            continue
        }
        // Never put temporary Office/ODF outputs beside the user's documents.
        if tempDir == "" {
            folder, e := os.MkdirTemp("", "official-pdf-manual-*")
            if e != nil { return 0, e }
            tempDir = folder
        }
        converted := filepath.Join(tempDir, fmt.Sprintf("%04d.pdf", i))
        if err := convertEditable(input, converted); err != nil {
            return 0, fmt.Errorf("%s 轉換失敗：%w", filepath.Base(input), err)
        }
        if err := verifyConvertedPDF(converted); err != nil {
            return 0, fmt.Errorf("%s 轉換結果無效：%w", filepath.Base(input), err)
        }
        pdfs = append(pdfs, converted)
    }
        if report!=nil {report(5+55*(i+1)/len(inputs),"正在準備及轉換文件")}
    }
    if len(pdfs) == 1 {
        // A single editable document can still be dropped onto the EXE to
        // convert it. Preserve a single source PDF's bytes if requested.
        info, err := parsePDF(pdfs[0])
        if err != nil { return 0, err }
        src, err := os.Open(pdfs[0])
        if err != nil { return 0, err }
        defer src.Close()
        dst, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
        if err != nil { return 0, err }
        _, err = io.Copy(dst, src)
        closeErr := dst.Close()
        if err == nil { err = closeErr }
        if err != nil { os.Remove(output); return 0, err }
        return info.count, nil
    }
    return mergePDFFiles(pdfs, output)
}

// autoMergedPath returns an unused PDF path beside the first input. As Windows
// Explorer's selection order is not guaranteed to reflect desktop positions,
// callers must not silently re-sort the supplied inputs by filename.
func autoMergedPath(inputs []string) (string, error) {
    if len(inputs) == 0 { return "", errors.New("沒有文件") }
    if len(inputs) == 1 && strings.EqualFold(filepath.Ext(inputs[0]), ".pdf") {
        return "", errors.New("單一 PDF 不需要合併，請再加入其他文件")
    }
    base := "合併檔案.pdf"
    if len(inputs) == 1 {
        base = cleanComponent(strings.TrimSuffix(filepath.Base(inputs[0]), filepath.Ext(inputs[0]))) + ".pdf"
    }
    return autoOutputPath(inputs[0], base)
}

func verifyConvertedPDF(path string) error {
    f, err := os.Open(path)
    if err != nil { return fmt.Errorf("沒有產生 PDF：%w", err) }
    defer f.Close()
    var sig [5]byte
    n, err := f.Read(sig[:])
    if err != nil || n != len(sig) || string(sig[:]) != "%PDF-" {
        return errors.New("轉檔完成，但輸出不是有效的 PDF")
    }
    return nil
}
