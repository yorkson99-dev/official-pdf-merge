# Windows end-to-end tests use only synthetic PDF / dummy ODT content.
# No Office instance is required: unsafe mixed cases are cancelled *before*
# attempted conversion, ensuring no output file or Office automation begins.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class WarningE2E {
    [DllImport("user32.dll",CharSet=CharSet.Unicode)]
    public static extern IntPtr GetDlgItem(IntPtr h, int id);
    [DllImport("user32.dll",CharSet=CharSet.Unicode)]
    public static extern IntPtr FindWindow(string cls, string title);
    [DllImport("user32.dll",CharSet=CharSet.Unicode)]
    public static extern int GetWindowText(IntPtr h, System.Text.StringBuilder text, int max);
    [DllImport("user32.dll")]
    public static extern IntPtr SendMessage(IntPtr h, uint msg, UIntPtr w, IntPtr l);
    [DllImport("user32.dll")]
    [return:MarshalAs(UnmanagedType.Bool)]
    public static extern bool PostMessage(IntPtr h, uint msg, UIntPtr w, IntPtr l);
}
'@
$root=Join-Path $env:RUNNER_TEMP 'official-pdf-mixed-warning-e2e'
New-Item -ItemType Directory -Force $root | Out-Null
$env:FASTPDF_STARTUP_LOG=Join-Path $root 'warning-startup-trace.log'
Remove-Item $env:FASTPDF_STARTUP_LOG -Force -ErrorAction SilentlyContinue
$exe=Join-Path $PSScriptRoot '公文PDF快合.exe'
$registry='HKCU:\Software\OfficialPDFMerge'
$previousExists=$false; $oldValue=$null
if (Test-Path $registry) {
    $original=Get-ItemProperty -Path $registry -Name ShowMixedFormatWarning -ErrorAction SilentlyContinue
    if ($null -ne $original) {
        $previousExists=$true
        $oldValue=[int]$original.ShowMixedFormatWarning
    }
}
function Start-GUI([string[]]$files) {
    if ($files.Count -eq 0) { $p=Start-Process -FilePath $exe -PassThru }
    else { $p=Start-Process -FilePath $exe -ArgumentList $files -PassThru }
    for ($i=0;$i -lt 50;$i++) {
        Start-Sleep -Milliseconds 180
        $p.Refresh()
        if ($p.HasExited) {throw "EXE exited unexpectedly: $($p.ExitCode)"}
        if ($p.MainWindowHandle -ne [IntPtr]::Zero) {
            $control=[WarningE2E]::GetDlgItem($p.MainWindowHandle,112)
            if ($control -ne [IntPtr]::Zero){return $p}
        }
    }
    throw 'Warning checkbox/window did not appear'
}
function Assert-NoOutput([string]$folder) {
    if (Test-Path (Join-Path $folder '合併檔案.pdf')) {throw 'Cancel should not create any merged PDF'}
}
function Assert-DialogAndCancel([string[]]$files,[string]$folder) {
    $p=Start-GUI $files
    try {
        $caption='公文PDF快合｜慎用提醒'
        $dialog=[IntPtr]::Zero
        for ($i=0;$i -lt 80;$i++){
            Start-Sleep -Milliseconds 200
            $dialog=[WarningE2E]::FindWindow('FastPDFRiskDialog',$caption)
            if ($dialog -ne [IntPtr]::Zero){break}
        }
        if ($dialog -eq [IntPtr]::Zero){throw 'Expected warning dialog was not displayed'}
        Assert-NoOutput $folder
        $no=[WarningE2E]::GetDlgItem($dialog,1002)
        $yes=[WarningE2E]::GetDlgItem($dialog,1001)
        if ($no -eq [IntPtr]::Zero -or $yes -eq [IntPtr]::Zero){
            if (Test-Path $env:FASTPDF_STARTUP_LOG) {Get-Content $env:FASTPDF_STARTUP_LOG | Write-Output}
            throw "Custom native risk dialog is missing Stop/Continue buttons; Stop=$no Continue=$yes"
        }
        if (![WarningE2E]::PostMessage($dialog,0x111,[UIntPtr]::new(1002),$no)){
            throw 'Could not select Stop in TaskDialog'
        }
        for ($i=0;$i -lt 35;$i++){
            Start-Sleep -Milliseconds 200
            if ([WarningE2E]::FindWindow('FastPDFRiskDialog',$caption) -eq [IntPtr]::Zero){break}
        }
        if ([WarningE2E]::FindWindow('FastPDFRiskDialog',$caption) -ne [IntPtr]::Zero){throw 'Warning dialog failed to close on stop'}
        Assert-NoOutput $folder
        Write-Output 'PASS: mixed >3 warning shown, custom native dialog offers Stop and Continue, cancel produces no output.'
    } finally {
        $p.Refresh()
        if (!$p.HasExited){Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue}
    }
}
try {
    New-Item -Path $registry -Force | Out-Null
    Set-ItemProperty -Path $registry -Name ShowMixedFormatWarning -Type DWord -Value 1
    # E2E 1: checkbox starts on; toggling it uses HKCU and survives restarts.
    $p=Start-GUI @()
    try {
        $checkbox=[WarningE2E]::GetDlgItem($p.MainWindowHandle,112)
        $checked=[WarningE2E]::SendMessage($checkbox,0x00F0,[UIntPtr]::Zero,[IntPtr]::Zero)
        if ($checked.ToInt64() -ne 1){throw 'Safety warning checkbox should default to enabled'}
        [void][WarningE2E]::SendMessage($checkbox,0x00F5,[UIntPtr]::Zero,[IntPtr]::Zero)
        $enabled=(Get-ItemProperty -Path $registry -Name ShowMixedFormatWarning).ShowMixedFormatWarning
        if ($enabled -ne 0){throw "Disabled setting not stored in HKCU: $enabled"}
    } finally {
        $p.Refresh()
        if (!$p.HasExited){Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue}
    }
    $p=Start-GUI @()
    try {
        $checkbox=[WarningE2E]::GetDlgItem($p.MainWindowHandle,112)
        $checked=[WarningE2E]::SendMessage($checkbox,0x00F0,[UIntPtr]::Zero,[IntPtr]::Zero)
        if ($checked.ToInt64() -ne 0){throw 'Disabled checkbox preference was not reloaded'}
        [void][WarningE2E]::SendMessage($checkbox,0x00F5,[UIntPtr]::Zero,[IntPtr]::Zero)
        $enabled=(Get-ItemProperty -Path $registry -Name ShowMixedFormatWarning).ShowMixedFormatWarning
        if ($enabled -ne 1){throw 'Re-enabling warning was not persisted'}
    } finally {
        $p.Refresh()
        if (!$p.HasExited){Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue}
    }
    Write-Output 'PASS: warning is enabled by default and user checkbox preference persists in HKCU across restarts.'
    # E2E 2: ordinary files dragged onto EXE trigger modal before any Office activity.
    $mixed=Join-Path $root 'mixed'
    New-Item -ItemType Directory -Force $mixed | Out-Null
    $files=@()
    foreach($n in @(1,2,3)){
        $dest=Join-Path $mixed ("part$n.pdf")
        Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $dest
        $files+= $dest
    }
    $odt=Join-Path $mixed 'part4.odt'
    [System.IO.File]::WriteAllText($odt,'synthetic test only')
    $files+=$odt
    Assert-DialogAndCancel $files $mixed
    # E2E 3: ZIP counts only main and eligible attachments, ignoring opinion files.
    $payload=Join-Path $root 'zip-content'
    $letter=Join-Path $payload '來文'
    New-Item -ItemType Directory -Force $letter | Out-Null
    Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') (Join-Path $letter 'AB.pdf')
    Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') (Join-Path $letter 'AB_ATTCH1.pdf')
    Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') (Join-Path $letter 'AB_ATTCH2.pdf')
    Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') (Join-Path $letter 'AB(opinion).pdf')
    [System.IO.File]::WriteAllText((Join-Path $letter 'AB_ATTCH3.odt'),'synthetic test only')
    $archive=Join-Path $root 'mixed.zip'
    [System.IO.Compression.ZipFile]::CreateFromDirectory($payload,$archive)
    Assert-DialogAndCancel @($archive) $root
    # E2E 4: 13 pure PDFs must never show the caution, even with many files.
    $pure=Join-Path $root 'pure'
    New-Item -ItemType Directory -Force $pure | Out-Null
    $allPDF=@()
    for($n=1;$n -le 13;$n++){
        $dest=Join-Path $pure ("part$n.pdf")
        Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $dest
        $allPDF+=$dest
    }
    $p=Start-GUI $allPDF
    try {
        $done=$false
        for($i=0;$i -lt 110;$i++){
            Start-Sleep -Milliseconds 250
            if ([WarningE2E]::FindWindow('FastPDFRiskDialog','公文PDF快合｜慎用提醒') -ne [IntPtr]::Zero){
                throw 'Pure PDF merge must not show mixed-format warning'
            }
            $out=Join-Path $pure '合併檔案.pdf'
            if (Test-Path $out) {
                $done=$true
                break
            }
        }
        if (!$done){throw '13-file pure PDF merge did not create output'}
        Write-Output 'PASS: 13 pure PDFs merged with no mixed-format warning.'
    } finally {
        $p.Refresh()
        if (!$p.HasExited){Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue}
    }
    $sidecars=@(Get-ChildItem -Path $PSScriptRoot -File -Filter '*.ini' -ErrorAction SilentlyContinue)
    if ($sidecars.Count -gt 0){throw 'Unexpected INI file next to portable EXE'}
    Write-Output 'PASS: no INI settings file was produced next to portable executable.'
} finally {
    if ($previousExists){
        Set-ItemProperty -Path $registry -Name ShowMixedFormatWarning -Type DWord -Value $oldValue
    } else {
        Remove-ItemProperty -Path $registry -Name ShowMixedFormatWarning -ErrorAction SilentlyContinue
    }
}
