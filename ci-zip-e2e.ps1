# End-to-end ZIP auto-merge test on a GitHub-hosted Windows desktop.
# Only synthetic test PDFs. No real correspondence enters the repository.
$ErrorActionPreference = 'Stop'
$source = Join-Path $PSScriptRoot 'testdata/sample0.pdf'
$root = Join-Path $env:RUNNER_TEMP 'official-pdf-ci-e2e'
New-Item -ItemType Directory -Force $root | Out-Null
$payload = Join-Path $root 'zip-content'
$letter = Join-Path $payload '來文'
New-Item -ItemType Directory -Force $letter | Out-Null
Copy-Item $source (Join-Path $letter '100_200.pdf')
Copy-Item $source (Join-Path $letter '100_200_ATTCH2.pdf')
Copy-Item $source (Join-Path $letter '100_200_ATTCH1.pdf')
Copy-Item $source (Join-Path $letter '100_200(opinion).pdf')
Add-Type -AssemblyName System.IO.Compression.FileSystem
$archive = Join-Path $root 'smoke.zip'
if (Test-Path $archive) { Remove-Item $archive }
[System.IO.Compression.ZipFile]::CreateFromDirectory($payload, $archive)
$output = Join-Path $root 'smoke_來文資料待確認.pdf'
if (Test-Path $output) { Remove-Item $output }
Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public class Win32PDFSmoke {
  [DllImport("user32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
  public static extern IntPtr GetDlgItem(IntPtr hWnd, int id);
  [DllImport("user32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
  public static extern int GetWindowText(IntPtr hWnd, StringBuilder builder, int max);
  [DllImport("user32.dll", SetLastError=true)]
  public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint msg, UIntPtr wparam,
    IntPtr lparam, uint flags, uint timeout, out UIntPtr result);
}
'@
$env:FASTPDF_STARTUP_LOG = Join-Path $root 'win32-trace.log'
if (Test-Path $env:FASTPDF_STARTUP_LOG) { Remove-Item $env:FASTPDF_STARTUP_LOG }
$exe = Join-Path $PSScriptRoot '公文PDF快合.exe'
$p = Start-Process -FilePath $exe -ArgumentList @($archive) -PassThru
try {
  $passed = $false
  for ($i=0; $i -lt 70; $i++) {
    Start-Sleep -Milliseconds 350
    $p.Refresh()
    if ($p.HasExited) { throw "App exited unexpectedly while merging ZIP, exit=$($p.ExitCode)" }
    $hwnd = $p.MainWindowHandle
    if ($hwnd -eq [IntPtr]::Zero) { continue }
    $statusHandle = [Win32PDFSmoke]::GetDlgItem($hwnd, 202)
    if ($statusHandle -eq [IntPtr]::Zero) { continue }
    $text = New-Object System.Text.StringBuilder 4096
    [void][Win32PDFSmoke]::GetWindowText($statusHandle, $text, $text.Capacity)
    if (!(Test-Path $output) -or !($text.ToString().Contains('已完成'))) { continue }
    $size = (Get-Item $output).Length
    if ($size -lt 100) { throw "Output PDF is unexpectedly short: $size" }
    [UIntPtr]$res = [UIntPtr]::Zero
    foreach ($command in @(112,111,112,111)) {
      $ok = [Win32PDFSmoke]::SendMessageTimeout($hwnd, 0x0111,
        [UIntPtr]::new([uint32]$command), [IntPtr]::Zero, 2, 2000, [ref]$res)
      if ($ok -eq [IntPtr]::Zero) {
        Write-Output "MODE SWITCH FAILURE, process responding=$($p.Responding):"
        if (Test-Path $env:FASTPDF_STARTUP_LOG) { Get-Content -Raw $env:FASTPDF_STARTUP_LOG | Write-Output }
        throw "GUI stopped responding to tab command $command after ZIP output"
      }
    }
    $passed = $true
    Write-Output "PASS: ZIP auto-merged beside source; success status delivered; GUI responded to four tab switches. PDF bytes=$size"
    break
  }
  if (!$passed) {
    Write-Output "FILES UNDER TEST DIRECTORY:"
    Get-ChildItem -Recurse $root | ForEach-Object { Write-Output ("  " + $_.FullName + " " + $_.Length) }
    Write-Output "STARTUP TRACE:"
    if (Test-Path $env:FASTPDF_STARTUP_LOG) { Get-Content -Raw $env:FASTPDF_STARTUP_LOG | Write-Output }
    $p.Refresh()
    Write-Output "PROCESS: responding=$($p.Responding), title=$($p.MainWindowTitle), window=$($p.MainWindowHandle)"
    $statusHandle = [Win32PDFSmoke]::GetDlgItem($p.MainWindowHandle, 202)
    $text = New-Object System.Text.StringBuilder 4096
    [void][Win32PDFSmoke]::GetWindowText($statusHandle, $text, $text.Capacity)
    Write-Output "STATUS: $($text.ToString())"
    throw "ZIP output or success status was not observed within 25 seconds"
  }
} finally {
  $p.Refresh()
  if (!$p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
}
