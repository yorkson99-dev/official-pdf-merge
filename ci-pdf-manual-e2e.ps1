# Integration smoke: add two PDF files via command line, then exercise manual
# ordering and list editing through actual Win32 messages (not just Go unit tests).
$ErrorActionPreference = 'Stop'
$root = Join-Path $env:RUNNER_TEMP 'official-pdf-manual-e2e'
New-Item -ItemType Directory -Force $root | Out-Null
$one = Join-Path $root 'first.pdf'
$two = Join-Path $root 'second.pdf'
Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $one -Force
Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $two -Force
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public class PDFManualWin32 {
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h, int id);
 [DllImport("user32.dll")] public static extern IntPtr SendMessageTimeout(IntPtr h, uint msg, UIntPtr w,
   IntPtr l, uint flags, uint ms, out UIntPtr result);
}
'@
$exe = Join-Path $PSScriptRoot '公文PDF快合.exe'
$p = Start-Process -FilePath $exe -ArgumentList @('--manual',$one,$two) -PassThru
try {
  $list = [IntPtr]::Zero
  for ($i=0; $i -lt 30; $i++) {
    Start-Sleep -Milliseconds 300
    $p.Refresh()
    if ($p.HasExited) { throw "Manual GUI exited unexpectedly, exit=$($p.ExitCode)" }
    if ($p.MainWindowHandle -ne [IntPtr]::Zero) {
      $list = [PDFManualWin32]::GetDlgItem($p.MainWindowHandle, 201)
      if ($list -ne [IntPtr]::Zero) { break }
    }
  }
  if ($list -eq [IntPtr]::Zero) { throw 'Manual PDF list not created' }
  [UIntPtr]$result = [UIntPtr]::Zero
  $ok = [PDFManualWin32]::SendMessageTimeout($list, 0x018B, [UIntPtr]::Zero, [IntPtr]::Zero, 2, 3000, [ref]$result)
  if ($ok -eq [IntPtr]::Zero -or $result.ToUInt64() -ne 2) {
    throw "Expected two PDFs on the manual list, got $($result.ToUInt64())"
  }
  [void][PDFManualWin32]::SendMessageTimeout($list, 0x0186, [UIntPtr]::new([uint32]1),
    [IntPtr]::Zero, 2, 3000, [ref]$result)
  foreach ($id in @(104,105,104,105,104,105,104,105,104,105,106)) {
    $ok = [PDFManualWin32]::SendMessageTimeout($p.MainWindowHandle, 0x0111,
      [UIntPtr]::new([uint32]$id), [IntPtr]::Zero, 2, 3000, [ref]$result)
    if ($ok -eq [IntPtr]::Zero) { throw "Manual PDF list froze on command $id" }
  }
  $ok = [PDFManualWin32]::SendMessageTimeout($list, 0x018B, [UIntPtr]::Zero,
    [IntPtr]::Zero, 2, 3000, [ref]$result)
  if ($ok -eq [IntPtr]::Zero -or $result.ToUInt64() -ne 1) {
    throw "Expected one PDF after remove, got $($result.ToUInt64())"
  }
  $ok = [PDFManualWin32]::SendMessageTimeout($p.MainWindowHandle, 0x0111,
    [UIntPtr]::new([uint32]107), [IntPtr]::Zero, 2, 3000, [ref]$result)
  if ($ok -eq [IntPtr]::Zero) { throw 'Manual PDF clear froze' }
  $ok = [PDFManualWin32]::SendMessageTimeout($list, 0x018B, [UIntPtr]::Zero,
    [IntPtr]::Zero, 2, 3000, [ref]$result)
  if ($ok -eq [IntPtr]::Zero -or $result.ToUInt64() -ne 0) { throw 'Clear list failed' }
  Write-Output 'PASS: manual PDF two-file add, repeated move, remove, clear and responsive window.'
} finally {
  $p.Refresh()
  if (!$p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
}
