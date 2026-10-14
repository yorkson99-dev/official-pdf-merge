//go:build windows

package main

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestNDCProfileAndArguments(t *testing.T) {
    profile:=`C:\Users\oa test\AppData\Local\Temp\ndc-test-profile`
    uri,err:=ndcProfileURI(profile)
    if err!=nil{t.Fatal(err)}
    if uri!="file:///C:/Users/oa%20test/AppData/Local/Temp/ndc-test-profile" {
        t.Fatalf("bad NDC profile file URL: %q",uri)
    }
    inputs:=[]string{`C:\tmp\0001.odt`,`C:\tmp\0002.xlsx`}
    args,err:=ndcArgs(profile,`C:\tmp\out`,inputs)
    if err!=nil{t.Fatal(err)}
    joined:=strings.Join(args,"|")
    for _,fragment:=range []string{
        "-env:UserInstallation="+uri,"--headless","--convert-to|pdf",
        `--outdir|C:\tmp\out`,`C:\tmp\0001.odt`,`C:\tmp\0002.xlsx`,
    }{
        if !strings.Contains(joined,fragment){t.Errorf("missing argument %q in %s",fragment,joined)}
    }
    if strings.Contains(strings.ToLower(joined),"powershell") {
        t.Fatal("converter must not call PowerShell")
    }
}

func TestNDCBatchProducesAllPDFsAndAvoidsDuplicateStems(t *testing.T) {
    root:=t.TempDir()
    src,_:=os.ReadFile(filepath.Join("testdata","sample0.pdf"))
    inputA:=filepath.Join(root,"folder-one","same.odt")
    inputB:=filepath.Join(root,"folder-two","same.docx")
    for _,input:=range []string{inputA,inputB} {
        if e:=os.MkdirAll(filepath.Dir(input),0700);e!=nil{t.Fatal(e)}
        if e:=os.WriteFile(input,[]byte("synthetic non-PDF document"),0600);e!=nil{t.Fatal(e)}
    }
    dests:=[]string{filepath.Join(root,"first.pdf"),filepath.Join(root,"second.pdf")}
    oldLoc,oldExec:=ndcLocateBinary,ndcExecute
    defer func(){ndcLocateBinary=oldLoc;ndcExecute=oldExec}()
    ndcLocateBinary=func()(string,error){return ndcDefaultBinary,nil}
    calls:=0
    profileDir:=""
    ndcExecute=func(ctx context.Context,exe string,args []string)(string,error){
        calls++
        if exe!=ndcDefaultBinary {t.Fatalf("wrong NDC exe: %s",exe)}
        var outDir string
        for i,a:=range args {
            if a=="--outdir"&&i+1<len(args){outDir=args[i+1]}
            if strings.HasPrefix(a,"-env:UserInstallation="){profileDir=strings.TrimPrefix(a,"-env:UserInstallation=")}
        }
        if outDir==""||profileDir==""{t.Fatal("no NDC output folder or profile")}
        for _,name:=range args[len(args)-2:] {
            stem:=strings.TrimSuffix(filepath.Base(name),filepath.Ext(name))
            if e:=os.WriteFile(filepath.Join(outDir,stem+".pdf"),src,0600);e!=nil{return "",e}
        }
        return "convert test files using filter : writer_pdf_Export",nil
    }
    if err:=convertDocumentsToPDFBatch([]string{inputA,inputB},dests);err!=nil {t.Fatal(err)}
    if calls!=1{t.Fatalf("expected one NDC invocation, got %d",calls)}
    for _,dst:=range dests {
        if err:=verifyConvertedPDF(dst);err!=nil{t.Fatal(err)}
        if pdf,err:=parsePDF(dst);err!=nil||pdf.count!=1{
            t.Fatalf("invalid NDC output %s: %v",dst,err)
        }
    }
}

func TestNDCMissingOutputDoesNotPublishPartialBatch(t *testing.T) {
    root:=t.TempDir()
    inputs:=[]string{filepath.Join(root,"one.odt"),filepath.Join(root,"two.odt")}
    dests:=[]string{filepath.Join(root,"one.pdf"),filepath.Join(root,"two.pdf")}
    for _,p:=range inputs {
        if err:=os.WriteFile(p,[]byte("synthetic"),0600);err!=nil{t.Fatal(err)}
    }
    oldLoc,oldExec:=ndcLocateBinary,ndcExecute
    defer func(){ndcLocateBinary=oldLoc;ndcExecute=oldExec}()
    ndcLocateBinary=func()(string,error){return ndcDefaultBinary,nil}
    src,err:=os.ReadFile(filepath.Join("testdata","sample0.pdf"))
    if err!=nil{t.Fatal(err)}
    ndcExecute=func(ctx context.Context,exe string,args []string)(string,error){
        var outDir string
        for i,a:=range args{
            if a=="--outdir"&&i+1<len(args){outDir=args[i+1]}
        }
        return "only one document completed",
            os.WriteFile(filepath.Join(outDir,"one.pdf"),src,0600)
    }
    if err:=convertDocumentsToPDFBatch(inputs,dests);err==nil{t.Fatal("missing output must fail")}
    for _,dest:=range dests {
        if _,err:=os.Stat(dest);!os.IsNotExist(err){t.Fatalf("unexpected partial output at %s",dest)}
    }
}
