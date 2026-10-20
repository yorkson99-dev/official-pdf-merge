package main

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestGuidedGeneralTwoManualStagesAndFinish(t *testing.T){
 old:=convertEditable
 sample,e:=os.ReadFile("testdata/sample0.pdf");if e!=nil{t.Fatal(e)}
 convertEditable=func(in,out string)error{return os.WriteFile(out,sample,0600)}
 defer func(){convertEditable=old}()
 dir:=t.TempDir()
 inputs:=[]string{}
 for _,name:=range []string{"a.pdf","b.odt","c.pdf","d.docx","e.ods","f.pdf"}{
  path:=filepath.Join(dir,name)
  data:=sample
  if !strings.EqualFold(filepath.Ext(path),".pdf"){data=[]byte("synthetic")}
  if e=os.WriteFile(path,data,0600);e!=nil{t.Fatal(e)}
  inputs=append(inputs,path)
 }
 one,e:=runGeneralGuidedStage(inputs)
 if e!=nil||one.Finished||one.Converted!=2||one.Completed!=4{
  t.Fatalf("first stage %+v %v",one,e)
 }
 two,e:=runGeneralGuidedStage(inputs)
 if e!=nil||!two.Finished||!two.Published||two.Converted!=1||two.Pages!=6{
  t.Fatalf("second stage %+v %v",two,e)
 }
 pdf,e:=parsePDF(two.PDFPath)
 if e!=nil||pdf.count!=6{t.Fatalf("published PDF invalid %v",e)}
 again,e:=runGeneralGuidedStage(inputs)
 if e!=nil||again.PDFPath!=two.PDFPath{t.Fatalf("duplicate output on retry %+v %v",again,e)}
}
func TestGuidedGeneralRejectsChangedSelectedFile(t *testing.T){
 old:=convertEditable
 sample,e:=os.ReadFile("testdata/sample0.pdf");if e!=nil{t.Fatal(e)}
 convertEditable=func(in,out string)error{return os.WriteFile(out,sample,0600)}
 defer func(){convertEditable=old}()
 dir:=t.TempDir();files:=[]string{}
 for _,n:=range []string{"1.odt","2.docx","3.ods"}{
  p:=filepath.Join(dir,n);if e=os.WriteFile(p,[]byte("synthetic"),0600);e!=nil{t.Fatal(e)}
  files=append(files,p)
 }
 first,e:=runGeneralGuidedStage(files);if e!=nil{t.Fatal(e)}
 if e=os.WriteFile(files[2],[]byte("modified"),0600);e!=nil{t.Fatal(e)}
 if _,e=runGeneralGuidedStage(files);e==nil||!strings.Contains(e.Error(),"不一致"){
  t.Fatalf("changed file must fail: %v",e)
 }
 state,_,e:=loadGuidedCheckpoint(first.CheckpointPath)
 if e!=nil||state.Stage!=1{t.Fatal("previous checkpoint changed")}
}
