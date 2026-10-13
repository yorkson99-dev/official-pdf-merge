# Create the icon at build time and attach it as a native Win32 resource.
# No additional program or package is required on the user's PC.
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class NativeIconResource {
 [DllImport("kernel32.dll", SetLastError=true, CharSet=CharSet.Unicode)]
 public static extern IntPtr BeginUpdateResource(string fileName, bool deleteExistingResources);
 [DllImport("kernel32.dll", SetLastError=true)]
 public static extern bool UpdateResource(IntPtr handle, IntPtr resourceType, IntPtr resourceName, ushort language, byte[] data, uint length);
 [DllImport("kernel32.dll", SetLastError=true)]
 public static extern bool EndUpdateResource(IntPtr handle, bool discard);
}
'@
function Add-RoundedRect($graphics, $brush, [int]$x, [int]$y, [int]$w, [int]$h, [int]$r) {
 $path = New-Object System.Drawing.Drawing2D.GraphicsPath
 $d = $r*2
 $path.AddArc($x,$y,$d,$d,180,90)
 $path.AddArc($x+$w-$d,$y,$d,$d,270,90)
 $path.AddArc($x+$w-$d,$y+$h-$d,$d,$d,0,90)
 $path.AddArc($x,$y+$h-$d,$d,$d,90,90)
 $path.CloseFigure()
 $graphics.FillPath($brush,$path)
 $path.Dispose()
}
# 256x256 master scaled down with high-quality anti-aliasing for shell icon.
$bitmap = New-Object System.Drawing.Bitmap(256,256)
$g=[System.Drawing.Graphics]::FromImage($bitmap)
$g.SmoothingMode=[System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$g.Clear([System.Drawing.Color]::Transparent)
$base=[System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(255,16,36,77))
$blue=[System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(255,58,149,245))
$white=[System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(255,255,255,255))
$ice=[System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(255,218,241,255))
$red=[System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(255,243,70,82))
$cyan=[System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(255,14,203,221))
Add-RoundedRect $g $base 12 12 232 232 53
# Three offset files emerging from a ZIP.
Add-RoundedRect $g $blue 41 50 131 146 17
Add-RoundedRect $g $ice 62 33 130 157 16
Add-RoundedRect $g $white 83 60 129 153 18
$g.FillPolygon($red, [System.Drawing.Point[]]@(
 [System.Drawing.Point]::new(172,60),[System.Drawing.Point]::new(212,100),
 [System.Drawing.Point]::new(172,100)))
$thin=[System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(255,166,190,218),9)
$thin.StartCap=[System.Drawing.Drawing2D.LineCap]::Round
$thin.EndCap=[System.Drawing.Drawing2D.LineCap]::Round
$g.DrawLine($thin,107,123,175,123)
$g.DrawLine($thin,107,151,161,151)
$wide=[System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(255,14,203,221),19)
$wide.StartCap=[System.Drawing.Drawing2D.LineCap]::Round
$wide.EndCap=[System.Drawing.Drawing2D.LineCap]::Round
$g.DrawLine($wide,38,187,130,187)
$g.DrawLine($wide,122,187,159,160)
$g.DrawLine($wide,122,187,159,214)
$g.Dispose()
$pngPath=Join-Path $PSScriptRoot 'app-icon-64.png'
$small = New-Object System.Drawing.Bitmap(64,64)
$gg=[System.Drawing.Graphics]::FromImage($small)
$gg.InterpolationMode=[System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$gg.SmoothingMode=[System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$gg.DrawImage($bitmap,0,0,64,64)
$gg.Dispose()
$small.Save($pngPath,[System.Drawing.Imaging.ImageFormat]::Png)
$small.Dispose()
$bitmap.Dispose()
foreach ($brush in @($base,$blue,$white,$ice,$red,$cyan)){$brush.Dispose()}
$wide.Dispose();$thin.Dispose()
$png=[System.IO.File]::ReadAllBytes($pngPath)
# ICO format permits PNG-compressed payloads (Vista+).
$icoPath=Join-Path $PSScriptRoot 'app-icon.ico'
$icoHeader=New-Object byte[] 22
$icoHeader[2]=1;$icoHeader[4]=1;$icoHeader[6]=64;$icoHeader[7]=64
$icoHeader[10]=1;$icoHeader[12]=32
[BitConverter]::GetBytes([uint32]$png.Length).CopyTo($icoHeader,14)
[BitConverter]::GetBytes([uint32]22).CopyTo($icoHeader,18)
[System.IO.File]::WriteAllBytes($icoPath, $icoHeader+$png)
# RT_ICON 3, RT_GROUP_ICON 14, ID 1.
$group=New-Object byte[] 20
$group[2]=1;$group[4]=1;$group[6]=64;$group[7]=64
$group[10]=1;$group[12]=32
[BitConverter]::GetBytes([uint32]$png.Length).CopyTo($group,14)
[BitConverter]::GetBytes([uint16]1).CopyTo($group,18)
$exe=Join-Path $PSScriptRoot '公文PDF快合.exe'
$handle=[NativeIconResource]::BeginUpdateResource($exe,$false)
if ($handle -eq [IntPtr]::Zero) { throw "BeginUpdateResource: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())" }
try {
 if (![NativeIconResource]::UpdateResource($handle,[IntPtr]3,[IntPtr]1,1033,$png,[uint32]$png.Length)) { throw 'Failed adding RT_ICON' }
 if (![NativeIconResource]::UpdateResource($handle,[IntPtr]14,[IntPtr]1,1033,$group,[uint32]$group.Length)) { throw 'Failed adding RT_GROUP_ICON' }
} catch {
 [void][NativeIconResource]::EndUpdateResource($handle,$true)
 throw
}
if (![NativeIconResource]::EndUpdateResource($handle,$false)) { throw 'EndUpdateResource failed' }
Write-Output "PASS: native icon embedded: $($png.Length) PNG bytes"
