package main

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestStreamingPDFWritesDeferredPageTreeAndValidTrailer(t *testing.T) {
    data,err:=os.ReadFile("testdata/sample0.pdf")
    if err!=nil{t.Fatal(err)}
    root:=t.TempDir()
    output:=filepath.Join(root,"streamed.pdf")
    stream,err:=newPDFStreamMerge(output)
    if err!=nil{t.Fatal(err)}
    defer stream.abort()
    for i:=0;i<12;i++{
        doc,e:=parsePDFBytes(data,"synthetic.pdf")
        if e!=nil{t.Fatal(e)}
        if e=stream.appendPDF(doc);e!=nil{t.Fatal(e)}
    }
    count,err:=stream.commit()
    if err!=nil{t.Fatal(err)}
    if count!=12{t.Fatalf("pages = %d, want 12",count)}
    merged,err:=os.ReadFile(output)
    if err!=nil{t.Fatal(err)}
    // Object 2 is emitted after source objects, not rewritten in-place.
    if !bytes.Contains(merged,[]byte("/Pages 2 0 R")){t.Fatal("missing catalog page root")}
    tree:=bytes.Index(merged,[]byte("2 0 obj\n<< /Type /Pages /Count 12 /Kids ["))
    if tree<0 || tree<len(merged)/3{t.Fatal("deferred Pages object missing or written too early")}
    if !bytes.HasSuffix(merged,[]byte("%%EOF\n")){t.Fatal("missing PDF EOF")}
    pdf,err:=parsePDF(output)
    if err!=nil{t.Fatal(err)}
    if pdf.count!=12{t.Fatalf("parsed PDF has %d pages",pdf.count)}
    entries,err:=os.ReadDir(root)
    if err!=nil{t.Fatal(err)}
    if len(entries)!=1 || entries[0].Name()!="streamed.pdf"{t.Fatalf("temporary files remain: %v",entries)}
}

func TestStreamingMergeRemovesIncompleteTempAndKeepsSources(t *testing.T) {
    valid,err:=os.ReadFile("testdata/sample0.pdf")
    if err!=nil{t.Fatal(err)}
    root:=t.TempDir()
    first:=filepath.Join(root,"valid.pdf")
    second:=filepath.Join(root,"broken.pdf")
    output:=filepath.Join(root,"final.pdf")
    if err=os.WriteFile(first,valid,0600);err!=nil{t.Fatal(err)}
    if err=os.WriteFile(second,[]byte("%PDF-1.7\ninvalid body\n"),0600);err!=nil{t.Fatal(err)}
    if _,err=mergePDFFiles([]string{first,second},output);err==nil{t.Fatal("invalid second PDF unexpectedly accepted")}
    if _,err=os.Stat(output);!os.IsNotExist(err){t.Fatalf("partial output published: %v",err)}
    matches,err:=filepath.Glob(filepath.Join(root,".fastpdfmerge-*.tmp"))
    if err!=nil||len(matches)!=0{t.Fatalf("abandoned temp files: %v (%v)",matches,err)}
    b,err:=os.ReadFile(first)
    if err!=nil || !bytes.Equal(b,valid){t.Fatal("original input was modified")}
}

func TestStreamingMergeRejectsExistingOutput(t *testing.T){
    original:=[]byte("do not overwrite")
    path:=filepath.Join(t.TempDir(),"existing.pdf")
    if err:=os.WriteFile(path,original,0600);err!=nil{t.Fatal(err)}
    if _,err:=newPDFStreamMerge(path);err==nil{t.Fatal("existing output should be protected")}
    b,err:=os.ReadFile(path)
    if err!=nil||strings.TrimSpace(string(b))!="do not overwrite"{t.Fatal("overwrote existing file")}
}
