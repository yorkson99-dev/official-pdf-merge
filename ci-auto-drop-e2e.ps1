# Windows E2E: two ordinary PDFs dropped onto an unopened EXE must merge
# beside the first source without ever showing a Save As dialog.
$ErrorActionPreference = 'Stop'
$root = Join-Path $env:RUNNER_TEMP 'official-pdf-autodrop-e2e'
New-Item -ItemType Directory -Force $root | Out-Null
$one = Join-Path $root 'alpha.pdf'
$two = Join-Path $root 'beta.pdf'
Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $one -Force
Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $two -Force
$out = Join-Path $root '合併檔案.pdf'
Remove-Item $out -Force -ErrorAction SilentlyContinue
$exe = Join-Path $PSScriptRoot '公文PDF快合.exe'
$env:FASTPDF_STARTUP_LOG = Join-Path $root 'auto-startup.log'
Remove-Item $env:FASTPDF_STARTUP_LOG -Force -ErrorAction SilentlyContinue
Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class AutoDropWin32 {
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h, int id);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h, StringBuilder s, int len);
 [DllImport("user32.dll")] public static extern IntPtr SendMessageTimeout(IntPtr h,uint msg,UIntPtr w,IntPtr l,uint flags,uint timeout,out UIntPtr result);
}
'@
$p = Start-Process -FilePath $exe -ArgumentList @($one,$two) -PassThru
try {
  $passed = $false
  for ($i=0; $i -lt 60; $i++) {
    Start-Sleep -Milliseconds 350
    $p.Refresh()
    if ($p.HasExited) { throw "Program exited with code $($p.ExitCode)" }
    $h=$p.MainWindowHandle
    if ($h -eq [IntPtr]::Zero) { continue }
    $statusH=[AutoDropWin32]::GetDlgItem($h,202)
    if ($statusH -eq [IntPtr]::Zero) { continue }
    $text=New-Object System.Text.StringBuilder 4096
    [void][AutoDropWin32]::GetWindowText($statusH,$text,$text.Capacity)
    if (!(Test-Path $out) -or !$text.ToString().Contains('完成')) { continue }
    if ($text.ToString().Contains($root)) {throw "Full output directory leaked into status and may wrap"}
    if ($text.Length -gt 58) {throw "Status too long: $($text.Length)"}
    [UIntPtr]$result=[UIntPtr]::Zero
    $ok=[AutoDropWin32]::SendMessageTimeout($h,0, [UIntPtr]::Zero,[IntPtr]::Zero,2,2000,[ref]$result)
    if ($ok -eq [IntPtr]::Zero) {throw 'GUI became unresponsive after auto-merge'}
    $passed=$true
    Write-Output "PASS: direct EXE multi-file drop auto-merged with no Save As, status length $($text.Length), responsive window."
    break
  }
  if (!$passed) {
    Write-Output 'TRACE:'
    if (Test-Path $env:FASTPDF_STARTUP_LOG){Get-Content -Raw $env:FASTPDF_STARTUP_LOG | Write-Output}
    throw 'Auto-drop PDF output not confirmed in 21 seconds'
  }
  $next=Join-Path $root '合併檔案 (2).pdf'
  Remove-Item $next -Force -ErrorAction SilentlyContinue
  # A second run must never overwrite the first output.
  $second=Start-Process -FilePath $exe -ArgumentList @($one,$two) -PassThru
  try {
    for ($i=0;$i -lt 55;$i++) {
      Start-Sleep -Milliseconds 300
      if (Test-Path $next) {break}
    }
    if (!(Test-Path $next)) {throw 'Collision-safe second auto output was not produced'}
    Write-Output 'PASS: repeated file drop uses numbered output and preserves previous PDF.'
  } finally {
    $second.Refresh()
    if (!$second.HasExited) {Stop-Process -Id $second.Id -Force -ErrorAction SilentlyContinue}
  }
} finally {
  $p.Refresh()
  if (!$p.HasExited) {Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue}
}
