package main

import (
 "archive/zip"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestZipEditableAttachmentsAreOrderedAndNotSilentlyDropped(t *testing.T) {
 src, err := os.ReadFile(filepath.Join("testdata", "sample0.pdf"))
 if err != nil { t.Fatal(err) }
 root := t.TempDir()
 archive := filepath.Join(root, "sample.zip")
 f, err := os.Create(archive)
 if err != nil { t.Fatal(err) }
 zw := zip.NewWriter(f)
 entries := []struct{name string; body []byte}{
  {"來文/7_9_ATTCH10.pptx", []byte("mock pptx")},
  {"來文/7_9_ATTCH2.docx", []byte("mock docx")},
  {"來文/7_9_ATTCH1.odt", []byte("mock odt")},
  {"來文/7_9.pdf", src},
  {"來文/7_9(opinion).pdf",src},
  {"稿/draft.pdf",src},
 }
 for _, e := range entries {
  w, err := zw.Create(e.name)
  if err != nil { t.Fatal(err) }
  if _, err := w.Write(e.body); err != nil { t.Fatal(err) }
 }
 if err := zw.Close(); err != nil { t.Fatal(err) }
 if err := f.Close(); err != nil { t.Fatal(err) }
 job, err := analyzeZIP(archive)
 if err != nil { t.Fatal(err) }
 if len(job.Entries) != 4 { t.Fatalf("got %d entries, want 4",len(job.Entries)) }
 expected := []string{"7_9.pdf","7_9_ATTCH1.odt","7_9_ATTCH2.docx","7_9_ATTCH10.pptx"}
 for i,x := range expected { if job.Entries[i].name!=x { t.Errorf("order %d: %s != %s",i,job.Entries[i].name,x) } }
 old := convertEditable
 defer func(){convertEditable=old}()
 var encountered []string
 convertEditable = func(input, out string) error {
  encountered=append(encountered,strings.ToLower(filepath.Ext(input)))
  return os.WriteFile(out,src,0600)
 }
 target := filepath.Join(root,"merged.pdf")
 pages,err := mergeOfficialZIP(job,target)
 if err!=nil {t.Fatal(err)}
 if pages!=4 {t.Fatalf("pages=%d, want 4",pages)}
 wantExt:=[]string{".odt",".docx",".pptx"}
 for i,e := range wantExt {
  if encountered[i] != e {t.Fatalf("conversions=%v",encountered)}
 }
 converted,err:=parsePDF(target)
 if err!=nil {t.Fatal(err)}
 if converted.count!=4 {t.Fatalf("merged PDF count %d", converted.count)}
}

func TestZipRejectsUnsupportedAttachment(t *testing.T) {
 root := t.TempDir()
 p := filepath.Join(root,"unknown.zip")
 f,err:=os.Create(p);if err!=nil{t.Fatal(err)}
 z:=zip.NewWriter(f)
 for _,name:=range []string{"來文/a.pdf","來文/a_ATTCH1.weird"} {
  w,err:=z.Create(name);if err!=nil {t.Fatal(err)}
  _,err=io.WriteString(w,"content");if err!=nil {t.Fatal(err)}
 }
 z.Close();f.Close()
 if _,err:=analyzeZIP(p);err==nil {t.Fatal("unsupported attachment must raise an error")}
}

func TestKnownEditableExtensions(t *testing.T) {
 exts:=[]string{".doc",".docx",".docm",".odt",".rtf",".txt",".xls",".xlsx",".xlsm",".ods",".csv",".tsv",".ppt",".pptx",".pptm",".odp"}
 for _,e:=range exts{if !supportedOfficialExt(e){t.Errorf("missing %s",e)}}
 if supportedOfficialExt(".exe"){t.Fatal("must not treat executable as document")}
 _=fmt.Sprintf
}
