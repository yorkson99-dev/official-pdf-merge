# Real Win32 E2E: test-only diagnostic option writes a cumulative PDF and
# checkpoint, presents a completion notice, then EXITs after the user accepts.
# Synthetic PDFs only. No Office or antivirus policy changes on CI.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class DiagWin32 {
 [DllImport("user32.dll",CharSet=CharSet.Unicode)]
 public static extern IntPtr FindWindow(string cls,string title);
 [DllImport("user32.dll")]
 public static extern IntPtr GetDlgItem(IntPtr hwnd,int id);
 [DllImport("user32.dll")]
 public static extern bool PostMessage(IntPtr hwnd,uint msg,UIntPtr wp,IntPtr lp);
}
'@
$root=Join-Path $env:RUNNER_TEMP 'official-checkpoint-diagnostic-e2e'
$payload=Join-Path $root 'payload'
$incoming=Join-Path $payload '來文'
New-Item -ItemType Directory -Path $incoming -Force | Out-Null
$sample=Join-Path $PSScriptRoot 'testdata/sample0.pdf'
Copy-Item $sample (Join-Path $incoming 'a.pdf') -Force
Copy-Item $sample (Join-Path $incoming 'a_ATTCH1.pdf') -Force
Copy-Item $sample (Join-Path $incoming 'a_ATTCH2.pdf') -Force
$zip=Join-Path $root 'case.zip'
if(Test-Path $zip){Remove-Item $zip -Force}
[System.IO.Compression.ZipFile]::CreateFromDirectory($payload,$zip)
$out=Join-Path $root 'case.分段診斷第001階段.pdf'
$state=Join-Path $root 'case.分段診斷.json'
Remove-Item $out,$state -Force -ErrorAction SilentlyContinue
$exe=Join-Path $PSScriptRoot '公文PDF快合.exe'
$proc=Start-Process -FilePath $exe -ArgumentList "--diagnostic-zip `"$zip`"" -PassThru
try{
 $created=$false
 for($i=0;$i -lt 100;$i++){
  Start-Sleep -Milliseconds 200
  $proc.Refresh()
  if($proc.HasExited){throw "Diagnostic EXE unexpectedly exited before completion: $($proc.ExitCode)"}
  if((Test-Path $out)-and(Test-Path $state)){$created=$true;break}
 }
 if(!$created){throw 'Diagnostic did not publish cumulative PDF and JSON checkpoint'}
 $st=Get-Content -Raw -LiteralPath $state -Encoding UTF8 | ConvertFrom-Json
 if($st.stage -ne 1 -or $st.completed -ne 3 -or !$st.finished -or $st.pages -ne 3){
  throw "Invalid checkpoint contents: $(Get-Content -Raw $state)"
 }
 if((Get-Item $out).Length -lt 100){throw 'Diagnostic cumulative PDF is invalid'}
 # The diagnostic GUI deliberately terminates after a short visible status,
 # without a blocking message box or automatic child-process restart.
 for($i=0;$i -lt 70;$i++){
  Start-Sleep -Milliseconds 150
  $proc.Refresh()
  if($proc.HasExited){break}
 }
 if(!$proc.HasExited){throw 'Diagnostic process did not close automatically after stage completion'}
 if($proc.ExitCode -ne 0){throw "Diagnostic process exit code $($proc.ExitCode)"}
 Write-Output 'PASS: launched test-only ZIP mode, wrote verified cumulative PDF plus JSON, terminated the whole process automatically.'
}finally{
 $proc.Refresh()
 if(!$proc.HasExited){Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue}
}
