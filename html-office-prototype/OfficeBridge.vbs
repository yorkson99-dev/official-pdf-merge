' Local Office conversion worker for Microsoft Office.
' Standard Windows Script Host, no HTTP server, no PowerShell, no new EXE.
' Must be launched deliberately with cscript by the visible .cmd launcher.
' Requests are only accepted inside ./OfficeBridgeData/jobs/j<32 hex chars>.
Option Explicit
Dim fso, ws, folderRoot, jobsRoot, validator, running
Set fso=CreateObject("Scripting.FileSystemObject")
Set ws=CreateObject("WScript.Shell")
folderRoot=fso.BuildPath(fso.GetParentFolderName(WScript.ScriptFullName),"OfficeBridgeData")
If Not fso.FolderExists(folderRoot) Then fso.CreateFolder folderRoot
jobsRoot=fso.BuildPath(folderRoot,"jobs")
If Not fso.FolderExists(jobsRoot) Then fso.CreateFolder jobsRoot
Set validator=New RegExp
validator.Pattern="^j[0-9a-f]{32}$"
validator.IgnoreCase=False
running=True

WScript.Echo "Office Bridge v0.1.0 - localhost only (file exchange)"
WScript.Echo "Waiting for HTML conversion requests. Keep this window open."
WScript.Echo "Bridge folder: " & folderRoot
WScript.Echo "Press Ctrl+C / close console to stop."
Do While running
    Dim jobs,job
    Set jobs=fso.GetFolder(jobsRoot)
    For Each job In jobs.SubFolders
        If validator.Test(job.Name) Then
            If fso.FileExists(fso.BuildPath(job.Path,"request.txt")) Then
                If Not fso.FileExists(fso.BuildPath(job.Path,"busy.flag")) And Not fso.FileExists(fso.BuildPath(job.Path,"result.txt")) Then
                    HandleJob job.Path
                End If
            End If
        End If
    Next
    WScript.Sleep 800
Loop

Sub WriteAscii(path,value)
    Dim stream
    Set stream=fso.CreateTextFile(path,True,False)
    stream.Write value
    stream.Close
End Sub

Function Allowed(name)
    Dim regex
    Set regex=New RegExp
    regex.Pattern="^[0-9]{4}\.(doc|docx|docm|odt|rtf|txt|xls|xlsx|xlsm|ods|csv|tsv|ppt|pptx|pptm|odp)$"
    regex.IgnoreCase=False
    Allowed=regex.Test(name)
End Function

Function ExtOf(name)
    ExtOf=LCase(fso.GetExtensionName(name))
End Function

Function ConvertOffice(src, dest, ext, ByRef wordApp, ByRef excelApp, ByRef pptApp)
    On Error Resume Next
    Dim doc,books,pres,number,message
    ConvertOffice=False
    Err.Clear
    If ext="doc" Or ext="docx" Or ext="docm" Or ext="odt" Or ext="rtf" Or ext="txt" Then
        If wordApp Is Nothing Then
            Set wordApp=CreateObject("Word.Application")
            If Err.Number<>0 Then Exit Function
            wordApp.Visible=False
            wordApp.DisplayAlerts=0
            wordApp.AutomationSecurity=3
            If Err.Number<>0 Then Exit Function
        End If
        Set doc=wordApp.Documents.Open(src,False,True,False)
        If Err.Number<>0 Then Exit Function
        doc.ExportAsFixedFormat dest,17
        number=Err.Number
        Err.Clear
        doc.Close 0
        Set doc=Nothing
        If number<>0 Then Exit Function
    ElseIf ext="xls" Or ext="xlsx" Or ext="xlsm" Or ext="ods" Or ext="csv" Or ext="tsv" Then
        If excelApp Is Nothing Then
            Set excelApp=CreateObject("Excel.Application")
            If Err.Number<>0 Then Exit Function
            excelApp.Visible=False
            excelApp.DisplayAlerts=False
            excelApp.AskToUpdateLinks=False
            excelApp.AutomationSecurity=3
            If Err.Number<>0 Then Exit Function
        End If
        Set books=excelApp.Workbooks.Open(src,0,True)
        If Err.Number<>0 Then Exit Function
        books.ExportAsFixedFormat 0,dest
        number=Err.Number
        Err.Clear
        books.Close False
        Set books=Nothing
        If number<>0 Then Exit Function
    Else
        If pptApp Is Nothing Then
            Set pptApp=CreateObject("PowerPoint.Application")
            If Err.Number<>0 Then Exit Function
            pptApp.AutomationSecurity=3
            If Err.Number<>0 Then Exit Function
        End If
        Set pres=pptApp.Presentations.Open(src,-1,0,0)
        If Err.Number<>0 Then Exit Function
        pres.SaveAs dest,32
        number=Err.Number
        Err.Clear
        pres.Close
        Set pres=Nothing
        If number<>0 Then Exit Function
    End If
    ConvertOffice=fso.FileExists(dest)
    If ConvertOffice Then ConvertOffice=(fso.GetFile(dest).Size>8)
End Function

Sub HandleJob(jobPath)
    On Error Resume Next
    Dim inputDir,outputDir,request,result,reader,wordApp,excelApp,pptApp
    Dim name,ext,src,dest,i,parts,code,sessionCount
    inputDir=fso.BuildPath(jobPath,"input")
    outputDir=fso.BuildPath(jobPath,"output")
    WriteAscii fso.BuildPath(jobPath,"busy.flag"),"RUNNING"
    WScript.Echo "Job: " & fso.GetFileName(jobPath)
    result="DONE"
    If Not fso.FolderExists(inputDir) Or Not fso.FolderExists(outputDir) Then
        result="ERROR|MISSING_DIRECTORY"
    Else
        Set reader=fso.OpenTextFile(fso.BuildPath(jobPath,"request.txt"),1,False,0)
        If Err.Number<>0 Then
            result="ERROR|REQUEST_NOT_READABLE"
            Err.Clear
        Else
            Set wordApp=Nothing
            Set excelApp=Nothing
            Set pptApp=Nothing
            sessionCount=0
            Do Until reader.AtEndOfStream
                name=Trim(reader.ReadLine)
                If Len(name)>0 Then
                    sessionCount=sessionCount+1
                    If sessionCount>100 Then
                        result="ERROR|TOO_MANY_DOCUMENTS"
                        Exit Do
                    End If
                    If Not Allowed(name) Then
                        result="ERROR|INVALID_EXTENSION_OR_NAME"
                        Exit Do
                    End If
                    src=fso.BuildPath(inputDir,name)
                    ext=ExtOf(name)
                    dest=fso.BuildPath(outputDir,fso.GetBaseName(name)&".pdf")
                    If Not fso.FileExists(src) Then
                        result="ERROR|MISSING_INPUT|" & name
                        Exit Do
                    End If
                    WScript.Echo "  Office: " & name
                    Err.Clear
                    If Not ConvertOffice(src,dest,ext,wordApp,excelApp,pptApp) Then
                        code=Err.Number
                        Err.Clear
                        result="ERROR|OFFICE_CONVERSION|" & name & "|" & CStr(code)
                        Exit Do
                    End If
                End If
            Loop
            reader.Close
            If Not wordApp Is Nothing Then wordApp.Quit 0
            If Not excelApp Is Nothing Then excelApp.Quit
            If Not pptApp Is Nothing Then pptApp.Quit
            Set wordApp=Nothing
            Set excelApp=Nothing
            Set pptApp=Nothing
        End If
    End If
    WriteAscii fso.BuildPath(jobPath,"result.txt"),result
    WScript.Echo "  Result: " & result
    Err.Clear
End Sub
