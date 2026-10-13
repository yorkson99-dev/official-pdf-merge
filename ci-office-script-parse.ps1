# The hosted Windows runner does not have the user's Office installation.
# Verify that the bundled non-interactive Office conversion script parses,
# without attempting to access any real Office documents.
$ErrorActionPreference='Stop'
$source=Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'office_windows.go')
$mark='const officeConvertScript = `'
$start=$source.IndexOf($mark)
if ($start -lt 0) {throw 'Embedded Office script was not found'}
$start+=$mark.Length
$end=$source.IndexOf('`',$start)
if ($end -lt 0){throw 'Office script delimiter missing'}
$code=$source.Substring($start,$end-$start)
$tokens=$null
$parseErrors=$null
[void][System.Management.Automation.Language.Parser]::ParseInput($code,[ref]$tokens,[ref]$parseErrors)
if ($parseErrors.Count -gt 0) {
    foreach ($e in $parseErrors){Write-Output $e.ToString()}
    throw 'Office PowerShell converter script has syntax errors'
}
foreach($name in @('Word.Application','Excel.Application','PowerPoint.Application',
 'AutomationSecurity','ExportAsFixedFormat','SaveAs')) {
    if (!$code.Contains($name)){throw "Missing required conversion declaration: $name"}
}
Write-Output 'PASS: embedded Office conversion script parses and names all required COM APIs'
