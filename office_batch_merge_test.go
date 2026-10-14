package main

import (
    "archive/zip"
    "bytes"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

// All attachments are synthetic; no real official correspondence is committed.
func makeOfficeBatchZIP(t *testing.T, paths []string) string {
    t.Helper()
    sample,err:=os.ReadFile("testdata/sample0.pdf")
    if err!=nil{t.Fatal(err)}
    var buf bytes.Buffer
    zw:=zip.NewWriter(&buf)
    for _,path:=range paths {
        w,err:=zw.Create("來文/"+path)
        if err!=nil{t.Fatal(err)}
        data:=sample
        if !strings.EqualFold(filepath.Ext(path),".pdf"){data=[]byte("synthetic Office test")}
        if _,err=w.Write(data);err!=nil{t.Fatal(err)}
    }
    if err=zw.Close();err!=nil{t.Fatal(err)}
    archive:=filepath.Join(t.TempDir(),"office-batch.zip")
    if err=os.WriteFile(archive,buf.Bytes(),0600);err!=nil{t.Fatal(err)}
    return archive
}

func TestTwelveFilesMixedOfficePDFBatches(t *testing.T) {
    paths:=[]string{"2026_500.pdf"}
    for i:=1;i<=11;i++ {
        ext:=".pdf"
        if i==3 {ext=".docx"}
        if i==6 || i==9 {ext=".odt"}
        paths=append(paths,fmt.Sprintf("2026_500_ATTCH%d%s",i,ext))
    }
    job,err:=analyzeZIP(makeOfficeBatchZIP(t,paths))
    if err!=nil{t.Fatal(err)}
    if len(job.Entries)!=12 {t.Fatalf("expected 12 entries, got %d",len(job.Entries))}
    for i:=1;i<=11;i++ {
        if job.Entries[i].number!=i{t.Fatalf("attachment order broken at %d: %s",i,job.Entries[i].name)}
    }
    src,err:=os.ReadFile("testdata/sample0.pdf")
    if err!=nil{t.Fatal(err)}
    oldBatch:=convertEditableBatch
    oldSingle:=convertEditable
    defer func(){convertEditableBatch=oldBatch;convertEditable=oldSingle}()
    batchCalls:=0
    seen:=[]string{}
    convertEditableBatch=func(inputs,outputs []string)error{
        batchCalls++
        if len(inputs)>officeBatchLimit {return errors.New("batch limit exceeded")}
        for i,input:=range inputs{
            seen=append(seen,filepath.Ext(input))
            if err:=os.WriteFile(outputs[i],src,0600);err!=nil{return err}
        }
        return nil
    }
    convertEditable=func(input,output string)error{
        t.Fatal("three editable items should use one batch, not single conversions")
        return nil
    }
    outDir:=t.TempDir()
    out:=filepath.Join(outDir,"combined.pdf")
    pages,err:=mergeOfficialZIP(job,out)
    if err!=nil{t.Fatal(err)}
    if batchCalls!=1 {t.Fatalf("wanted one Word batch, got %d",batchCalls)}
    if got:=strings.Join(seen,",");got!=".docx,.odt,.odt"{t.Fatalf("unexpected Office order %s",got)}
    if pages!=12 {t.Fatalf("expected 12 pages, got %d",pages)}
    parsed,err:=parsePDF(out)
    if err!=nil || parsed.count!=12 {t.Fatalf("output invalid: %v",err)}
    files,err:=os.ReadDir(outDir)
    if err!=nil{t.Fatal(err)}
    if len(files)!=1 || files[0].Name()!="combined.pdf" {
        t.Fatalf("intermediate merge outputs found: %v",files)
    }
}

func TestOfficeBatchLimitAndNoPartialOutput(t *testing.T){
    paths:=[]string{"2026_777.pdf"}
    for i:=1;i<=7;i++{paths=append(paths,fmt.Sprintf("2026_777_ATTCH%d.odt",i))}
    job,err:=analyzeZIP(makeOfficeBatchZIP(t,paths))
    if err!=nil{t.Fatal(err)}
    src,err:=os.ReadFile("testdata/sample0.pdf")
    if err!=nil{t.Fatal(err)}
    oldBatch,oldSingle:=convertEditableBatch,convertEditable
    defer func(){convertEditableBatch=oldBatch;convertEditable=oldSingle}()
    calls:=[]int{}
    convertEditableBatch=func(inputs,outputs []string) error {
        calls=append(calls,len(inputs))
        for i:=range inputs{
            if err:=os.WriteFile(outputs[i],src,0600);err!=nil{return err}
        }
        return nil
    }
    convertEditable=func(input,output string)error{
        calls=append(calls,1)
        return os.WriteFile(output,src,0600)
    }
    out:=filepath.Join(t.TempDir(),"eight.pdf")
    pages,err:=mergeOfficialZIP(job,out)
    if err!=nil{t.Fatal(err)}
    if pages!=8||fmt.Sprint(calls)!="[3 3 1]"{
        t.Fatalf("unexpected stages=%v pages=%d",calls,pages)
    }
    convertEditableBatch=func(inputs,outputs []string)error{return errors.New("synthetic Office failure")}
    failed:=filepath.Join(t.TempDir(),"not-created.pdf")
    if _,err:=mergeOfficialZIP(job,failed);err==nil{t.Fatal("expected failure")}
    if _,err:=os.Stat(failed);!os.IsNotExist(err){t.Fatalf("partial output unexpectedly exists: %v",err)}
}
