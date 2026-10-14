package main

import (
    "archive/zip"
    "bytes"
    "errors"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func makeSyntheticOfficialZIP(t *testing.T, filenames []string) string {
    t.Helper()
    src, err := os.ReadFile(filepath.Join("testdata", "sample0.pdf"))
    if err != nil { t.Fatal(err) }
    var buf bytes.Buffer
    zw := zip.NewWriter(&buf)
    for _, name := range filenames {
        w, e := zw.Create(name)
        if e != nil { t.Fatal(e) }
        payload := src
        if strings.HasSuffix(name, ".odt") {
            payload = []byte("ODF test input: synthetic, not a real document")
        }
        if _, e = w.Write(payload); e != nil { t.Fatal(e) }
    }
    if e := zw.Close(); e != nil { t.Fatal(e) }
    archive := filepath.Join(t.TempDir(), "synthetic.zip")
    if e := os.WriteFile(archive, buf.Bytes(), 0600); e != nil { t.Fatal(e) }
    return archive
}

func TestPDFOnlyZIPNeverCreatesDocumentWorkspace(t *testing.T) {
    archive := makeSyntheticOfficialZIP(t, []string{
        "來文/X_202.pdf",
        "來文/X_202_ATTCH3.pdf",
        "來文/X_202_ATTCH1.pdf",
        "來文/X_202(opinion).pdf",
        "稿/draft.pdf",
    })
    job, err := analyzeZIP(archive)
    if err != nil { t.Fatal(err) }
    old := createZIPWorkspace
    defer func(){ createZIPWorkspace = old }()
    createZIPWorkspace = func(dir, pattern string) (string,error) {
        t.Errorf("a pure PDF ZIP must not create an input workspace")
        return "", errors.New("unnecessary workspace")
    }
    out := filepath.Join(t.TempDir(), "merged.pdf")
    pages, err := mergeOfficialZIP(job, out)
    if err != nil { t.Fatal(err) }
    if pages != 3 { t.Fatalf("want three PDF pages, got %d", pages) }
    doc, err := parsePDF(out)
    if err != nil { t.Fatal(err) }
    if doc.count != 3 { t.Fatalf("output PDF reports %d pages", doc.count) }
    if _, err := mergeOfficialZIP(job, out); err == nil {
        t.Fatal("existing official PDF must not be overwritten")
    }
}

func TestOnePDFZIPPreservesOriginalBytesWithoutWorkspace(t *testing.T) {
    archive := makeSyntheticOfficialZIP(t, []string{"來文/X_77.pdf"})
    job, err := analyzeZIP(archive)
    if err != nil { t.Fatal(err) }
    old := createZIPWorkspace
    defer func(){ createZIPWorkspace = old }()
    createZIPWorkspace = func(dir, pattern string) (string,error) {
        return "", errors.New("unexpected scratch creation")
    }
    out := filepath.Join(t.TempDir(), "one.pdf")
    pages, err := mergeOfficialZIP(job,out)
    if err != nil || pages != 1 {t.Fatalf("single main: %d %v",pages,err)}
    original, err := os.ReadFile(filepath.Join("testdata","sample0.pdf"))
    if err != nil {t.Fatal(err)}
    got,err := os.ReadFile(out)
    if err != nil {t.Fatal(err)}
    if !bytes.Equal(original,got) {t.Fatal("one-file merge must preserve source PDF byte-for-byte")}
}

func TestEditableZIPCreatesWorkspaceOnlyOnce(t *testing.T) {
    archive := makeSyntheticOfficialZIP(t, []string{
        "來文/X_202.pdf",
        "來文/X_202_ATTCH1.odt",
        "來文/X_202_ATTCH2.pdf",
    })
    job,err:=analyzeZIP(archive)
    if err != nil {t.Fatal(err)}
    oldWorkspace:=createZIPWorkspace
    oldConvert:=convertEditable
    defer func(){createZIPWorkspace=oldWorkspace;convertEditable=oldConvert}()
    count:=0
    createZIPWorkspace=func(folder,pattern string)(string,error){
        count++
        return os.MkdirTemp(folder,pattern)
    }
    source,err:=os.ReadFile(filepath.Join("testdata","sample0.pdf"))
    if err!=nil{t.Fatal(err)}
    converted:=0
    convertEditable=func(input,output string) error {
        converted++
        if filepath.Ext(input)!=".odt" {return errors.New("unexpected format")}
        return os.WriteFile(output,source,0600)
    }
    out:=filepath.Join(t.TempDir(),"mixed.pdf")
    pages,err:=mergeOfficialZIP(job,out)
    if err!=nil{t.Fatal(err)}
    if pages!=3 || converted!=1 || count!=1 {
        t.Fatalf("pages=%d conversions=%d workspace=%d",pages,converted,count)
    }
    doc,err:=parsePDF(out)
    if err!=nil || doc.count!=3 {t.Fatalf("merged PDF verification: %v",err)}
}

func TestConversionFailureDoesNotLeaveOutput(t *testing.T) {
    archive:=makeSyntheticOfficialZIP(t,[]string{
        "來文/X_202.pdf","來文/X_202_ATTCH1.odt",
    })
    job,err:=analyzeZIP(archive)
    if err!=nil{t.Fatal(err)}
    old:=convertEditable
    defer func(){convertEditable=old}()
    convertEditable=func(input,output string) error {return errors.New("Office unavailable")}
    out:=filepath.Join(t.TempDir(),"should-not-exist.pdf")
    if _,err:=mergeOfficialZIP(job,out);err==nil {t.Fatal("conversion must fail")}
    if _,err:=os.Stat(out);!os.IsNotExist(err){t.Fatalf("unexpected partial output: %v",err)}
}

func TestParsePDFBytesMatchesFileParser(t *testing.T) {
    raw,err:=os.ReadFile(filepath.Join("testdata","sample0.pdf"))
    if err!=nil{t.Fatal(err)}
    fileParsed,err:=parsePDF(filepath.Join("testdata","sample0.pdf"))
    if err!=nil{t.Fatal(err)}
    memParsed,err:=parsePDFBytes(raw,"in-memory-pdf")
    if err!=nil{t.Fatal(err)}
    if fileParsed.count!=memParsed.count || len(fileParsed.objects)!=len(memParsed.objects){
        t.Fatalf("memory parser differs: file pages=%d objects=%d, memory pages=%d objects=%d",
            fileParsed.count,len(fileParsed.objects),memParsed.count,len(memParsed.objects))
    }
}
