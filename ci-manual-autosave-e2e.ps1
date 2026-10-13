# Win32 end-to-end: ordered manual list defaults to same-folder PDF output.
# The Save As dialog is an explicit opt-in checkbox, initially unchecked.
$ErrorActionPreference = 'Stop'
$root = Join-Path $env:RUNNER_TEMP 'official-pdf-manual-autosave'
New-Item -ItemType Directory -Force $root | Out-Null
$one = Join-Path $root 'one.pdf'
$two = Join-Path $root 'two.pdf'
Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $one -Force
Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $two -Force
$out = Join-Path $root '合併檔案.pdf'
Remove-Item $out -Force -ErrorAction SilentlyContinue
Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class ManualAutoSaveWin32 {
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h, int id);
 [DllImport("user32.dll")] public static extern IntPtr SendMessageTimeout(IntPtr h,uint msg,UIntPtr w,IntPtr l,uint flags,uint milliseconds,out UIntPtr result);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h, StringBuilder buf,int len);
}
'@
$exe = Join-Path $PSScriptRoot '公文PDF快合.exe'
$p = Start-Process -FilePath $exe -ArgumentList @('--manual',$one,$two) -PassThru
try {
 $hwnd=[IntPtr]::Zero
 for($i=0;$i -lt 35;$i++){
   Start-Sleep -Milliseconds 250
   $p.Refresh()
   if($p.HasExited){throw "GUI terminated early: $($p.ExitCode)"}
   if($p.MainWindowHandle -ne [IntPtr]::Zero){$hwnd=$p.MainWindowHandle;break}
 }
 if($hwnd -eq [IntPtr]::Zero){throw 'No main window'}
 $checkbox=[ManualAutoSaveWin32]::GetDlgItem($hwnd,111)
 if($checkbox -eq [IntPtr]::Zero){throw 'Optional Save As checkbox missing'}
 [UIntPtr]$result=[UIntPtr]::Zero
 $checkOk=[ManualAutoSaveWin32]::SendMessageTimeout($checkbox,0x00F0,[UIntPtr]::Zero,[IntPtr]::Zero,2,1500,[ref]$result)
 if($checkOk -eq [IntPtr]::Zero -or $result.ToUInt64() -ne 0){throw 'Save As must be unchecked by default'}
 $clickOk=[ManualAutoSaveWin32]::SendMessageTimeout($hwnd,0x0111,[UIntPtr]::new([uint32]108),[IntPtr]::Zero,2,1800,[ref]$result)
 if($clickOk -eq [IntPtr]::Zero){throw 'Manual merge command blocked unexpectedly'}
 $passed=$false
 for($i=0;$i -lt 65;$i++){
   Start-Sleep -Milliseconds 300
   $p.Refresh()
   if($p.HasExited){throw "GUI exited during manual merge"}
   $status=[ManualAutoSaveWin32]::GetDlgItem($hwnd,202)
   $txt=New-Object System.Text.StringBuilder 1024
   [void][ManualAutoSaveWin32]::GetWindowText($status,$txt,$txt.Capacity)
   if((Test-Path $out) -and $txt.ToString().Contains('完成')){ $passed=$true;break }
 }
 if(!$passed){throw 'Manual merge did not save beside first source without Save As'}
 Write-Output 'PASS: manual ordered merge auto-saves to source folder with default unchecked Save As.'
 $toggle=[ManualAutoSaveWin32]::SendMessageTimeout($checkbox,0x00F5,[UIntPtr]::Zero,[IntPtr]::Zero,2,1800,[ref]$result)
 if($toggle -eq [IntPtr]::Zero){throw 'Save As checkbox did not respond to click'}
 $checked=[ManualAutoSaveWin32]::SendMessageTimeout($checkbox,0x00F0,[UIntPtr]::Zero,[IntPtr]::Zero,2,1800,[ref]$result)
 if($checked -eq [IntPtr]::Zero -or $result.ToUInt64() -eq 0){throw 'Save As option cannot be enabled'}
 Write-Output 'PASS: Save As remains available as an explicit opt-in.'
} finally {
 $p.Refresh()
 if(!$p.HasExited){Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue}
}
