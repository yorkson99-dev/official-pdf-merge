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

func makeDiagnosticFixture(t *testing.T,root string)string{
 t.Helper()
 sample,e:=os.ReadFile("testdata/sample0.pdf")
 if e!=nil{t.Fatal(e)}
 dest:=filepath.Join(root,"diagnostic.zip")
 f,e:=os.Create(dest)
 if e!=nil{t.Fatal(e)}
 writer:=zip.NewWriter(f)
 names:=[]string{
  "來文/case.pdf",
  "來文/case_ATTCH1.odt",
  "來文/case_ATTCH2.pdf",
  "來文/case_ATTCH3.docx",
  "來文/case_ATTCH4.pdf",
  "來文/case_ATTCH5.ods",
  "來文/case_ATTCH6.pdf",
  "來文/case(opinion).pdf",
 }
 for _,name:=range names{
  child,e:=writer.Create(name)
  if e!=nil{t.Fatal(e)}
  data:=sample
  if strings.HasSuffix(name,".odt")||strings.HasSuffix(name,".docx")||strings.HasSuffix(name,".ods"){
   data=[]byte("synthetic editable test only")
  }
  if _,e=child.Write(data);e!=nil{t.Fatal(e)}
 }
 if e=writer.Close();e!=nil{t.Fatal(e)}
 if e=f.Close();e!=nil{t.Fatal(e)}
 return dest
}

func TestDiagnosticBoundaryAllowsOnlyTwoOfficeInputs(t *testing.T){
 j:=[]officialEntry{
  {name:"main.pdf"},{name:"a.odt"},{name:"b.pdf"},
  {name:"c.docx"},{name:"d.pdf"},{name:"e.ods"},{name:"f.pdf"},
 }
 first,edits:=guidedStageBoundary(j,0)
 if first!=5||edits!=2{t.Fatalf("first end=%d conversions=%d, want 5/2",first,edits)}
 second,edits:=guidedStageBoundary(j,5)
 if second!=7||edits!=1{t.Fatalf("second end=%d conversions=%d, want 7/1",second,edits)}
}

func TestDiagnosticTwoProcessStagesRecoverFromPersistedCheckpoint(t *testing.T){
 original:=convertEditable
 defer func(){convertEditable=original}()
 sample,e:=os.ReadFile("testdata/sample0.pdf")
 if e!=nil{t.Fatal(e)}
 calls:=[]string{}
 convertEditable=func(input,output string)error{
  calls=append(calls,filepath.Ext(input))
  return os.WriteFile(output,sample,0600)
 }
 dir:=t.TempDir()
 zipPath:=makeDiagnosticFixture(t,dir)
 one,e:=runZIPGuidedStage(zipPath)
 if e!=nil{t.Fatal(e)}
 if one.Stage!=1||one.Converted!=2||one.Completed!=5||one.Total!=7||one.Finished||one.Pages!=5 {
  t.Fatalf("first stage result invalid: %+v",one)
 }
 firstData,e:=os.ReadFile(one.PDFPath)
 if e!=nil{t.Fatal(e)}
 if !bytes.HasPrefix(firstData,[]byte("%PDF-")){t.Fatal("first stage is not PDF")}
 if _,e=os.Stat(one.CheckpointPath);e!=nil{t.Fatal(e)}
 if len(calls)!=2{t.Fatalf("first stage converted %d, want 2",len(calls))}
 state,ok,e:=loadGuidedCheckpoint(one.CheckpointPath)
 if e!=nil||!ok||state.Completed!=5{t.Fatalf("checkpoint missing: %+v (%v)",state,e)}

 // The second call simulates a completely new process: only on-disk JSON
 // and a persisted cumulative PDF carry the state forward.
 two,e:=runZIPGuidedStage(zipPath)
 if e!=nil{t.Fatal(e)}
 if two.Stage!=2||two.Converted!=1||two.Completed!=7||!two.Finished||two.Pages!=7{
  t.Fatalf("second stage result invalid: %+v",two)
 }
 if len(calls)!=3{t.Fatalf("duplicate conversion across processes: %v",calls)}
 parsed,e:=parsePDF(two.PDFPath)
 if e!=nil||parsed.count!=7{t.Fatalf("final PDF invalid: %v",e)}
 originalPDF,e:=os.ReadFile(one.PDFPath)
 if e!=nil||!bytes.Equal(firstData,originalPDF){t.Fatal("earlier stage PDF was modified")}
 again,e:=runZIPGuidedStage(zipPath)
 if e!=nil||!again.Finished||again.Stage!=2||len(calls)!=3{
  t.Fatalf("finished test must not reconvert documents: %+v (%v)",again,e)
 }
}

func TestDiagnosticRejectsChangedSourceAndBrokenCumulativePDF(t *testing.T){
 original:=convertEditable
 defer func(){convertEditable=original}()
 sample,e:=os.ReadFile("testdata/sample0.pdf")
 if e!=nil{t.Fatal(e)}
 convertEditable=func(input,output string)error{return os.WriteFile(output,sample,0600)}
 dir:=t.TempDir()
 zipPath:=makeDiagnosticFixture(t,dir)
 first,e:=runZIPGuidedStage(zipPath)
 if e!=nil{t.Fatal(e)}
 f,e:=os.OpenFile(zipPath,os.O_APPEND|os.O_WRONLY,0600)
 if e!=nil{t.Fatal(e)}
 if _,e=f.Write([]byte("test-changed-source"));e!=nil{t.Fatal(e)}
 if e=f.Close();e!=nil{t.Fatal(e)}
 if _,e=runZIPGuidedStage(zipPath);e==nil||!strings.Contains(e.Error(),"不一致"){
  t.Fatalf("changed ZIP should stop resume: %v",e)
 }
 // Restore original ZIP for the PDF fingerprint test.
 zipPath=makeDiagnosticFixture(t,dir)
 if e=os.WriteFile(first.PDFPath,[]byte("damaged"),0600);e!=nil{t.Fatal(e)}
 if _,e=runZIPGuidedStage(zipPath);e==nil||!strings.Contains(e.Error(),"雜湊"){
  t.Fatalf("modified cumulative PDF should stop resume: %v",e)
 }
}

func TestDiagnosticFailureLeavesPreviousCheckpointIntact(t *testing.T){
 original:=convertEditable
 defer func(){convertEditable=original}()
 sample,e:=os.ReadFile("testdata/sample0.pdf")
 if e!=nil{t.Fatal(e)}
 convertEditable=func(input,output string)error{return os.WriteFile(output,sample,0600)}
 dir:=t.TempDir()
 zipPath:=makeDiagnosticFixture(t,dir)
 first,e:=runZIPGuidedStage(zipPath)
 if e!=nil{t.Fatal(e)}
 convertEditable=func(input,output string)error{return errors.New("synthetic Office failure")}
 if _,e=runZIPGuidedStage(zipPath);e==nil{t.Fatal("stage 2 must fail")}
 secondPath:=filepath.Join(dir,guidedZIPStagePDFName("diagnostic",2))
 if _,e=os.Stat(secondPath);!os.IsNotExist(e){t.Fatal("failed stage published output")}
 st,ok,e:=loadGuidedCheckpoint(first.CheckpointPath)
 if e!=nil||!ok||st.Stage!=1||st.Completed!=5{t.Fatalf("previous valid checkpoint lost: %+v %v",st,e)}
 if _,e=os.Stat(first.PDFPath);e!=nil{t.Fatal("previous stage PDF lost")}
}
