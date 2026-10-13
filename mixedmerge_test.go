package main

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestMixedMergeKeepsInputOrder(t *testing.T) {
    b, err := os.ReadFile(filepath.Join("testdata","sample0.pdf"))
    if err != nil {t.Fatal(err)}
    root := t.TempDir()
    a := filepath.Join(root,"first.odt")
    c := filepath.Join(root,"second.pdf")
    d := filepath.Join(root,"third.xlsx")
    if err=os.WriteFile(a, []byte("synthetic odt"),0600);err!=nil {t.Fatal(err)}
    for _,p := range []string{c,d} {
        data := b
        if p==d { data=[]byte("synthetic xlsx") }
        if err=os.WriteFile(p,data,0600);err!=nil{t.Fatal(err)}
    }
    old:=convertEditable
    defer func(){convertEditable=old}()
    var order []string
    convertEditable=func(src,dst string) error {
        order=append(order,filepath.Base(src))
        return os.WriteFile(dst,b,0600)
    }
    output:=filepath.Join(root,"output.pdf")
    count,err:=mergeMixedFiles([]string{a,c,d},output)
    if err!=nil {t.Fatal(err)}
    if count!=3 {t.Fatalf("pages=%d, want 3",count)}
    if strings.Join(order,",")!="first.odt,third.xlsx" {t.Fatalf("conversion order=%v",order)}
    pdf,err:=parsePDF(output)
    if err!=nil || pdf.count!=3 {t.Fatalf("bad merged PDF: %v",err)}
}
func TestAutoMergedPathAvoidsOverwrite(t *testing.T) {
    root:=t.TempDir()
    a:=filepath.Join(root,"test.docx")
    b:=filepath.Join(root,"next.pdf")
    out,err:=autoMergedPath([]string{a,b})
    if err!=nil {t.Fatal(err)}
    if filepath.Dir(out)!=root || filepath.Base(out)!="合併檔案.pdf" {t.Fatal(out)}
    if err=os.WriteFile(out,[]byte("exists"),0600);err!=nil{t.Fatal(err)}
    out,err=autoMergedPath([]string{a,b})
    if err!=nil || filepath.Base(out)!="合併檔案 (2).pdf" {t.Fatalf("%s %v",out,err)}
    one,err:=autoMergedPath([]string{a})
    if err!=nil || filepath.Base(one)!="test.pdf" {t.Fatalf("%s %v",one,err)}
    if _,err=autoMergedPath([]string{b});err==nil {t.Fatal("single PDF should not be auto-merged")}
}
func TestMixedMergeRejectsUnsupportedFile(t *testing.T) {
    root:=t.TempDir()
    _,err:=mergeMixedFiles([]string{filepath.Join(root,"unknown.exe")},filepath.Join(root,"out.pdf"))
    if err==nil {t.Fatal("must fail on unsupported extension")}
}
