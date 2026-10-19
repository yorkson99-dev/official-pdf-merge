package main

import (
 "errors"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func withSyntheticOffice(t *testing.T){
 t.Helper()
 old:=convertEditable
 sample,err:=os.ReadFile("testdata/sample0.pdf")
 if err!=nil{t.Fatal(err)}
 convertEditable=func(in,out string)error{return os.WriteFile(out,sample,0600)}
 t.Cleanup(func(){convertEditable=old})
}

func TestPublishedTwoStagesFinalNameAndIdempotentResume(t *testing.T){
 withSyntheticOffice(t)
 dir:=t.TempDir()
 zipPath:=makeDiagnosticFixture(t,dir)
 first,err:=runZIPPublishedStage(zipPath,false)
 if err!=nil{t.Fatal(err)}
 if first.Finished||first.Published||first.Stage!=1{t.Fatalf("first %+v",first)}
 second,err:=runZIPPublishedStage(zipPath,false)
 if err!=nil{t.Fatal(err)}
 if !second.Finished||!second.Published||second.Stage!=2||second.Pages!=7{t.Fatalf("final %+v",second)}
 if filepath.Base(second.PDFPath)!="diagnostic_來文資料待確認.pdf" {t.Fatal(second.PDFPath)}
 final,err:=parsePDF(second.PDFPath)
 if err!=nil||final.count!=7{t.Fatalf("published PDF invalid: %v",err)}
 state,ok,err:=loadDiagnosticCheckpoint(second.CheckpointPath)
 if err!=nil||!ok||state.FinalPDFPath==""||state.FinalPDFSHA256==""{t.Fatalf("missing final receipt: %+v %v",state,err)}
 for n:=1;n<=2;n++{
  file:=filepath.Join(dir,stagePDFName("diagnostic",n))
  if _,err=os.Stat(file);!os.IsNotExist(err){t.Fatalf("known stage %d not cleaned: %v",n,err)}
 }
 again,err:=runZIPPublishedStage(zipPath,false)
 if err!=nil||again.PDFPath!=second.PDFPath||!again.Published{t.Fatalf("replayed finished checkpoint %+v %v",again,err)}
}

func TestPublishedRejectsChangedFinalOutput(t *testing.T){
 withSyntheticOffice(t)
 dir:=t.TempDir()
 path:=makeDiagnosticFixture(t,dir)
 _,err:=runZIPPublishedStage(path,false)
 if err!=nil{t.Fatal(err)}
 done,err:=runZIPPublishedStage(path,false)
 if err!=nil{t.Fatal(err)}
 if err=os.WriteFile(done.PDFPath,[]byte("corrupted"),0600);err!=nil{t.Fatal(err)}
 if _,err=runZIPPublishedStage(path,false);err==nil||!strings.Contains(err.Error(),"雜湊"){t.Fatalf("changed final must reject: %v",err)}
}

func TestPublishedKeepsCheckpointOnPublishFailure(t *testing.T){
 withSyntheticOffice(t)
 dir:=t.TempDir()
 path:=makeDiagnosticFixture(t,dir)
 _,err:=runZIPPublishedStage(path,false)
 if err!=nil{t.Fatal(err)}
 original:=linkCheckpointOutput
 linkCheckpointOutput=func(string,string)error{return errors.New("unsupported volume")}
 defer func(){linkCheckpointOutput=original}()
 if _,err=runZIPPublishedStage(path,false);err==nil{t.Fatal("expected publish failure")}
 state,ok,err:=loadDiagnosticCheckpoint(filepath.Join(dir,"diagnostic.分段診斷.json"))
 if err!=nil||!ok||!state.Finished||state.FinalPDFPath!=""{t.Fatalf("failed publish damaged state %+v %v",state,err)}
 if _,err=os.Stat(filepath.Join(dir,stagePDFName("diagnostic",2)));err!=nil{t.Fatal(err)}
 linkCheckpointOutput=original
 final,err:=runZIPPublishedStage(path,false)
 if err!=nil||!final.Published{t.Fatalf("retry publish %+v %v",final,err)}
}

func TestPublishedDoesNotOverwriteExistingNamedPDF(t *testing.T){
 withSyntheticOffice(t)
 dir:=t.TempDir()
 path:=makeDiagnosticFixture(t,dir)
 old:=filepath.Join(dir,"diagnostic_來文資料待確認.pdf")
 if err:=os.WriteFile(old,[]byte("preexisting"),0600);err!=nil{t.Fatal(err)}
 _,err:=runZIPPublishedStage(path,false)
 if err!=nil{t.Fatal(err)}
 end,err:=runZIPPublishedStage(path,false)
 if err!=nil{t.Fatal(err)}
 if filepath.Base(end.PDFPath)!="diagnostic_來文資料待確認 (2).pdf"{t.Fatal(end.PDFPath)}
 data,err:=os.ReadFile(old)
 if err!=nil||string(data)!="preexisting"{t.Fatal("existing PDF overwritten")}
}
