//go:build windows

package main

import (
    "context"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

// Document conversion is invoked ONLY for non-PDF attachments. PDF-only ZIPs
// never start PowerShell or Microsoft Office.
func convertDocumentToPDF(input, output string) error {
    ext := strings.ToLower(filepath.Ext(input))
    if !supportedOfficialExt(ext) || ext == ".pdf" {
        return fmt.Errorf("不支援的可編輯檔格式：%s", ext)
    }
    // NDC ODF Application Tools can show a bootstrap.ini lock error when
    // launched headlessly in an existing office session. Microsoft Office has
    // successfully converted ODT on the target workstation, so prefer it.
    officeErr := convertViaOffice(input, output, ext)
    if officeErr == nil { return nil }
    // NDC fallback is deliberately OPT-IN, never run automatically: its
    // installer-specific bootstrap warning can block a zero-click workflow.
    if (ext == ".odt" || ext == ".ods" || ext == ".odp") &&
        os.Getenv("FASTPDF_ENABLE_NDC_FALLBACK") == "1" {
        if soffice := findNDCOffice(); soffice != "" {
            if ndcErr := convertViaODF(soffice, input, output); ndcErr == nil {
                return nil
            }
        }
    }
    return officeErr
}

func findNDCOffice() string {
    paths := []string{}
    if p, err := exec.LookPath("soffice.exe"); err == nil { paths = append(paths, p) }
    for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
        if root == "" { continue }
        for _, app := range []string{"NDC ODF", "NDC ODF Application Tools", "ODF文件應用工具", "國家發展委員會ODF文件應用工具", "LibreOffice"} {
            paths = append(paths, filepath.Join(root, app, "program", "soffice.exe"))
        }
    }
    for _, p := range paths {
        if st, e := os.Stat(p); e == nil && !st.IsDir() { return p }
    }
    return ""
}

func convertViaODF(soffice, input, output string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
    defer cancel()
    // Keep a separate headless profile to avoid locking the user's interactive
    // NDC ODF session and avoid modifying their preferences.
    profile, err := os.MkdirTemp("", "official-pdf-odf-profile-*")
    if err != nil { return err }
    defer os.RemoveAll(profile)
    profileURI := "file:///" + strings.ReplaceAll(filepath.ToSlash(profile), ":", "%3A")
    cmd := exec.CommandContext(ctx, soffice, "-env:UserInstallation="+profileURI, "--headless",
        "--convert-to", "pdf", "--outdir", filepath.Dir(output), input)
    cmd.WaitDelay = 3*time.Second
    data, err := cmd.CombinedOutput()
    if ctx.Err() != nil { return fmt.Errorf("NDC ODF 轉檔逾時") }
    if err != nil { return fmt.Errorf("NDC ODF 無法自動轉換：%w（%s）", err, strings.TrimSpace(string(data))) }
    defaultOutput := filepath.Join(filepath.Dir(output), strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))+".pdf")
    if _, err = os.Stat(defaultOutput); err != nil { return fmt.Errorf("NDC ODF 未產生 PDF") }
    if defaultOutput != output {
        if err = os.Rename(defaultOutput, output); err != nil { return err }
    }
    return verifyConvertedPDF(output)
}

// Use Windows PowerShell, which ships with Windows, to automate the already-
// installed Office COM applications. No Office installation occurs here.
// Do not override execution policy; organization security policy remains intact.
const officeConvertScript = `
$ErrorActionPreference = 'Stop'
$src=$env:OFFICIAL_PDF_SOURCE
$dst=$env:OFFICIAL_PDF_TARGET
$kind=$env:OFFICIAL_PDF_KIND
$app=$null
$document=$null
try {
    switch ($kind) {
        'word' {
            $app=New-Object -ComObject Word.Application
            $app.Visible=$false
            $app.DisplayAlerts=0
            $app.AutomationSecurity=3
            $document=$app.Documents.Open($src, $false, $true, $false)
            $document.ExportAsFixedFormat($dst, 17)
        }
        'excel' {
            $app=New-Object -ComObject Excel.Application
            $app.Visible=$false
            $app.DisplayAlerts=$false
            $app.AskToUpdateLinks=$false
            $app.AutomationSecurity=3
            $document=$app.Workbooks.Open($src, 0, $true)
            $document.ExportAsFixedFormat(0, $dst)
        }
        'powerpoint' {
            $app=New-Object -ComObject PowerPoint.Application
            $app.AutomationSecurity=3
            $document=$app.Presentations.Open($src, -1, 0, 0)
            $document.SaveAs($dst, 32)
        }
        default { throw "Unsupported Office family: $kind" }
    }
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
} finally {
    try {
        if ($null -ne $document) {
            if ($kind -eq 'excel') { $document.Close($false) }
            elseif ($kind -eq 'powerpoint') { $document.Close() }
            else { $document.Close(0) }
        }
    } catch {}
    try { if ($null -ne $app) { $app.Quit() } } catch {}
    if ($null -ne $document) { [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($document) }
    if ($null -ne $app) { [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($app) }
}
`

func convertViaOffice(input, output, ext string) error {
    kind := ""
    switch ext {
    case ".doc", ".docx", ".docm", ".odt", ".rtf", ".txt": kind = "word"
    case ".xls", ".xlsx", ".xlsm", ".ods", ".csv", ".tsv": kind = "excel"
    case ".ppt", ".pptx", ".pptm", ".odp": kind = "powerpoint"
    default: return errors.New("Office 無法處理此格式")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
    defer cancel()
    systemRoot := os.Getenv("SystemRoot")
    ps := "powershell.exe"
    if systemRoot != "" {
        p := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
        if _, e := os.Stat(p); e == nil { ps = p }
    }
    cmd := exec.CommandContext(ctx, ps, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", officeConvertScript)
    cmd.WaitDelay = 3*time.Second
    cmd.Env = append(os.Environ(), "OFFICIAL_PDF_SOURCE="+input,
        "OFFICIAL_PDF_TARGET="+output, "OFFICIAL_PDF_KIND="+kind)
    data, err := cmd.CombinedOutput()
    if ctx.Err() != nil { return fmt.Errorf("Office 轉檔逾時，請確認是否有權限提示視窗") }
    if err != nil {
        msg := strings.TrimSpace(string(data))
        if len(msg) > 350 { msg = msg[:350] }
        return fmt.Errorf("%s 無法匯出 PDF：%w（%s）", strings.ToUpper(kind), err, msg)
    }
    return verifyConvertedPDF(output)
}

