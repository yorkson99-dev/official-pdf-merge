# Windows GUI startup smoke test. Runs only in GitHub-hosted Windows runner.
$ErrorActionPreference = 'Stop'
$env:FASTPDF_STARTUP_LOG = Join-Path $PSScriptRoot 'startup-win32.log'
if (Test-Path $env:FASTPDF_STARTUP_LOG) { Remove-Item $env:FASTPDF_STARTUP_LOG }
$exe = Join-Path $PSScriptRoot '公文PDF快合.exe'
if (!(Test-Path $exe)) { throw "Missing exe: $exe" }
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public class Win32Window {
  [DllImport("user32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
  public static extern IntPtr FindWindow(string className, string windowName);
  [DllImport("user32.dll")]
  public static extern bool IsWindowVisible(IntPtr handle);
  [DllImport("user32.dll")]
  public static extern uint GetWindowThreadProcessId(IntPtr handle, out uint pid);
}
'@
$proc = Start-Process -FilePath $exe -PassThru
$seen = $false
try {
  for ($i = 0; $i -lt 20; $i++) {
    Start-Sleep -Milliseconds 500
    $proc.Refresh()
    if ($proc.HasExited) {
      Write-Output "GUI process exited, ExitCode=$($proc.ExitCode). Capturing diagnostics from console variant..."
      $dbgExe = Join-Path $PSScriptRoot 'debug-console.exe'
      $logOut = Join-Path $PSScriptRoot 'startup-stdout.log'
      $logErr = Join-Path $PSScriptRoot 'startup-stderr.log'
      $dbg = Start-Process -FilePath $dbgExe -PassThru -RedirectStandardOutput $logOut -RedirectStandardError $logErr
      Start-Sleep -Seconds 3
      $dbg.Refresh()
      if (!$dbg.HasExited) { Stop-Process -Id $dbg.Id -Force -ErrorAction SilentlyContinue }
      if (Test-Path $logOut) { Write-Output "STDOUT:"; Get-Content -Raw $logOut | Write-Output }
      if (Test-Path $logErr) { Write-Output "STDERR:"; Get-Content -Raw $logErr | Write-Output }
      throw "Process exited before showing a window. ExitCode=$($proc.ExitCode)"
    }
    # On hosted runners, FindWindow can be restricted by the window station.
    # System.Diagnostics.Process still resolves this process' own main HWND.
    if ($proc.MainWindowHandle -ne [IntPtr]::Zero -and $proc.MainWindowTitle -like '*快合 PDF*') {
      $seen = $true
      Write-Output "PASS: visible Win32 main window opened. Process ID=$($proc.Id), Handle=$($proc.MainWindowHandle), Title=$($proc.MainWindowTitle)"
      break
    }
    $hwnd = [Win32Window]::FindWindow('FastPDFMergeNativeGUI', $null)
    if ($hwnd -ne [IntPtr]::Zero) {
      [uint32]$windowPid = 0
      [void][Win32Window]::GetWindowThreadProcessId($hwnd, [ref]$windowPid)
      if ($windowPid -ne [uint32]$proc.Id) { continue }
      if (![Win32Window]::IsWindowVisible($hwnd)) { throw 'Window exists but is hidden' }
      $seen = $true
      Write-Output "PASS: visible Win32 window opened. Process ID=$($proc.Id), Handle=$hwnd"
      break
    }
  }
  if (!$seen) {
    Write-Output "Win32 startup trace:"
    if (Test-Path $env:FASTPDF_STARTUP_LOG) { Get-Content -Raw $env:FASTPDF_STARTUP_LOG | Write-Output }
    $proc.Refresh()
    Write-Output "Process state: ID=$($proc.Id), SessionID=$($proc.SessionId), MainWindowHandle=$($proc.MainWindowHandle), MainWindowTitle='$($proc.MainWindowTitle)'"
    throw "No Win32 main window appeared after 10 seconds. Process ID=$($proc.Id)"
  }
} finally {
  $proc.Refresh()
  if (!$proc.HasExited) {
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
  }
}
