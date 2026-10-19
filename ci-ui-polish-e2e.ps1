# Native Win32 UI smoke test for independent warning section, two real
# list columns, and completion of a true PDF merge progress bar.
$ErrorActionPreference='Stop'
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public static class PolishedUI {
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h,int id);
 [DllImport("user32.dll", CharSet=CharSet.Unicode)]
 public static extern int GetWindowText(IntPtr h,StringBuilder text,int max);
 [DllImport("user32.dll")] public static extern IntPtr SendMessage(
    IntPtr h,uint msg,UIntPtr w,IntPtr l);
 [DllImport("user32.dll")]
 public static extern bool PostMessage(IntPtr h,uint msg,UIntPtr w,IntPtr l);
 [DllImport("user32.dll")]
 public static extern bool GetWindowRect(IntPtr h,out WinRect rect);
}
[StructLayout(LayoutKind.Sequential)]
public struct WinRect {public int left,top,right,bottom;}
'@
function Title([IntPtr]$handle) {
    $b=New-Object System.Text.StringBuilder 512
    [void][PolishedUI]::GetWindowText($handle,$b,$b.Capacity)
    return $b.ToString()
}
$root=Join-Path $env:RUNNER_TEMP 'official-ui-polish-e2e'
New-Item -ItemType Directory -Force $root | Out-Null
$one=Join-Path $root 'one.pdf'
$two=Join-Path $root 'two.pdf'
Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $one -Force
Copy-Item (Join-Path $PSScriptRoot 'testdata/sample0.pdf') $two -Force
$out=Join-Path $root '合併檔案.pdf'
Remove-Item $out -Force -ErrorAction SilentlyContinue
$exe=Join-Path $PSScriptRoot '公文PDF快合.exe'
$p=Start-Process -FilePath $exe -ArgumentList @('--manual',$one,$two) -PassThru
try {
    $h=[IntPtr]::Zero
    for($i=0;$i -lt 50;$i++){
        Start-Sleep -Milliseconds 200
        $p.Refresh()
        if($p.HasExited){throw "Program unexpectedly exited: $($p.ExitCode)"}
        $h=$p.MainWindowHandle
        if($h -eq [IntPtr]::Zero){continue}
        $ordered=[PolishedUI]::GetDlgItem($h,204)
        $filename=[PolishedUI]::GetDlgItem($h,205)
        $progress=[PolishedUI]::GetDlgItem($h,203)
        $progressLabel=[PolishedUI]::GetDlgItem($h,206)
        $warning=[PolishedUI]::GetDlgItem($h,112)
        $list=[PolishedUI]::GetDlgItem($h,201)
        $merge=[PolishedUI]::GetDlgItem($h,108)
        if($ordered -ne [IntPtr]::Zero -and $filename -ne [IntPtr]::Zero -and
           $progress -ne [IntPtr]::Zero -and $progressLabel -ne [IntPtr]::Zero -and
           $warning -ne [IntPtr]::Zero -and $list -ne [IntPtr]::Zero -and $merge -ne [IntPtr]::Zero){
            break
        }
    }
    if($h -eq [IntPtr]::Zero -or $progress -eq [IntPtr]::Zero){throw 'Required native UI controls not found'}
    if((Title $ordered) -ne '序號' -or (Title $filename) -ne '檔案名稱'){
        throw "Incorrect list headers: $(Title $ordered) / $(Title $filename)"
    }
    $listCount=[PolishedUI]::SendMessage($list,0x018B,[UIntPtr]::Zero,[IntPtr]::Zero)
    if($listCount.ToInt64() -ne 2) {throw "Expected 2 manual files, got $($listCount.ToInt64())"}
    $warningRect=New-Object WinRect
    $mergeRect=New-Object WinRect
    [void][PolishedUI]::GetWindowRect($warning,[ref]$warningRect)
    [void][PolishedUI]::GetWindowRect($merge,[ref]$mergeRect)
    if($warningRect.top -le $mergeRect.bottom) {throw 'Shared warning toggle still inside the manual merge section'}
    Write-Output 'PASS: separate warning checkbox below both merge modes and true 序號 / 檔案名稱 headers.'
    if(![PolishedUI]::PostMessage($h,0x0111,[UIntPtr]::new(108),[IntPtr]::Zero)){
        throw 'Failed to submit merge button command'
    }
    $done=$false
    for($i=0;$i -lt 120;$i++){
        Start-Sleep -Milliseconds 200
        $percent=[PolishedUI]::SendMessage($progress,0x0408,[UIntPtr]::Zero,[IntPtr]::Zero)
        if((Test-Path $out) -and $percent.ToInt64() -eq 100 -and (Title $progressLabel) -eq '100%'){
            $done=$true
            break
        }
    }
    if(!$done){
        $current=[PolishedUI]::SendMessage($progress,0x0408,[UIntPtr]::Zero,[IntPtr]::Zero)
        throw "Progress did not reach 100 on successful merge: $($current.ToInt64()) / $(Title $progressLabel)"
    }
    Write-Output 'PASS: real Win32 progress bar reaches 100 percent only with merged PDF output.'
}finally{
    $p.Refresh()
    if(!$p.HasExited){Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue}
}
