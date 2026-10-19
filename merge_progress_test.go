package main

import (
 "archive/zip"
 "os"
 "path/filepath"
 "testing"
)

func checkProgressSequence(t *testing.T,values []int){
 t.Helper()
 if len(values)<2 {t.Fatalf("expected progress callbacks, got %v",values)}
 last:=0
 for i,p:=range values {
  if p<last || p>99 {t.Fatalf("invalid progress at %d: %v",i,values)}
  last=p
 }
 if last<95 {t.Fatalf("progress did not reach final pre-commit stage: %v",values)}
}

func TestPDFMergeProgressTracksEachInput(t *testing.T){
 bytes,err:=os.ReadFile("testdata/sample0.pdf")
 if err!=nil{t.Fatal(err)}
 dir:=t.TempDir()
 paths:=make([]string,6)
 for i:=range paths {
  p:=filepath.Join(dir,string(rune('a'+i))+".pdf")
  if err:=os.WriteFile(p,bytes,0600);err!=nil{t.Fatal(err)}
  paths[i]=p
 }
 percentages:=[]int{}
 output:=filepath.Join(dir,"result.pdf")
 pages,err:=mergeMixedFilesWithProgress(paths,output,func(percent int,stage string){
  if stage=="" {t.Fatal("empty stage")}
  percentages=append(percentages,percent)
 })
 if err!=nil{t.Fatal(err)}
 if pages!=6 {t.Fatalf("expected 6 pages; got %d",pages)}
 checkProgressSequence(t,percentages)
 pdf,err:=parsePDF(output)
 if err!=nil||pdf.count!=6{t.Fatalf("merged output invalid: %v",err)}
}

func TestOfficialZIPMergeProgressPreservesAttachmentOrder(t *testing.T){
 sample,err:=os.ReadFile("testdata/sample0.pdf")
 if err!=nil{t.Fatal(err)}
 path:=filepath.Join(t.TempDir(),"mixed-test.zip")
 f,err:=os.Create(path)
 if err!=nil{t.Fatal(err)}
 z:=zip.NewWriter(f)
 for _,name:=range []string{
  "來文/123.pdf","來文/123_ATTCH3.pdf","來文/123_ATTCH1.pdf","來文/123_ATTCH2.pdf",
 }{
  w,e:=z.Create(name)
  if e!=nil{t.Fatal(e)}
  if _,e=w.Write(sample);e!=nil{t.Fatal(e)}
 }
 if err=z.Close();err!=nil{t.Fatal(err)}
 if err=f.Close();err!=nil{t.Fatal(err)}
 job,err:=analyzeZIP(path)
 if err!=nil{t.Fatal(err)}
 out:=filepath.Join(t.TempDir(),"merged.pdf")
 values:=[]int{}
 pages,err:=mergeOfficialZIPWithProgress(job,out,func(n int,label string){
  if label==""{t.Fatal("progress stage was blank")}
  values=append(values,n)
 })
 if err!=nil{t.Fatal(err)}
 if pages!=4{t.Fatalf("4-page PDF expected, got %d",pages)}
 checkProgressSequence(t,values)
}
