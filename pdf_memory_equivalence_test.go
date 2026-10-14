package main

import (
    "bytes"
    "os"
    "path/filepath"
    "testing"
)

// Memory ZIP processing must not alter any merged PDF object, nor the exact
// output bytes produced by the established file-based merger.
func TestInMemoryAndDiskPDFMergeProduceIdenticalBytes(t *testing.T) {
    src, err := os.ReadFile(filepath.Join("testdata", "sample0.pdf"))
    if err != nil { t.Fatal(err) }
    dir := t.TempDir()
    input1 := filepath.Join(dir, "first.pdf")
    input2 := filepath.Join(dir, "second.pdf")
    for _, path := range []string{input1,input2} {
        if e:=os.WriteFile(path,src,0600);e!=nil {t.Fatal(e)}
    }
    diskOut := filepath.Join(dir,"disk.pdf")
    memOut := filepath.Join(dir,"memory.pdf")
    nDisk,err:=mergePDFFiles([]string{input1,input2},diskOut)
    if err!=nil{t.Fatal(err)}
    docs:=make([]*pdfInput,0,2)
    for i:=0;i<2;i++ {
        d,e:=parsePDFBytes(src,"memory-input")
        if e!=nil{t.Fatal(e)}
        docs=append(docs,d)
    }
    nMem,err:=mergePDFDocuments(docs,memOut)
    if err!=nil{t.Fatal(err)}
    a,err:=os.ReadFile(diskOut)
    if err!=nil{t.Fatal(err)}
    b,err:=os.ReadFile(memOut)
    if err!=nil{t.Fatal(err)}
    if nDisk!=nMem || !bytes.Equal(a,b) {
        t.Fatalf("memory and file PDF mergers differ: pages disk=%d mem=%d, bytes disk=%d mem=%d",
            nDisk,nMem,len(a),len(b))
    }
}
