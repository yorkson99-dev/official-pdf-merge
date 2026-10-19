package main

// Publish the final cumulative stage under the conventional official-letter
// filename only after complete PDF, page-count and SHA-256 verification.
// Retain a small checkpoint to prevent duplicate output and support verification;
// remove only intermediate stage PDFs whose hashes are known and match.
import (
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "strings"
 "time"
)

var linkCheckpointOutput = os.Link

func checkpointOutputName(job *officialJob,detailed bool)(string,bool){
 name,err:=job.outputName(detailed)
 if err==nil{return name,false}
 fallback:=cleanComponent(strings.TrimSuffix(filepath.Base(job.ZipPath),filepath.Ext(job.ZipPath)))
 if fallback==""{fallback="來文"}
 return fallback+"_來文資料待確認.pdf",true
}

func verifiedStageCopy(folder string,state diagnosticCheckpoint)(string,error){
 source:=filepath.Join(folder,state.PDFPath)
 origin,err:=os.Open(source)
 if err!=nil{return "",err}
 defer origin.Close()
 f,err:=os.CreateTemp(folder,".official-checkpoint-publish-*.pdf")
 if err!=nil{return "",err}
 name:=f.Name()
 cleanup:=true
 defer func(){if cleanup{_ =os.Remove(name)}}()
 _,err=io.Copy(f,origin)
 if err!=nil{_ =f.Close();return "",err}
 if err=f.Sync();err!=nil{_ =f.Close();return "",err}
 if err=f.Close();err!=nil{return "",err}
 hash,err:=fingerprintFile(name)
 if err!=nil{return "",err}
 if hash!=state.PDFSHA256{return "",errors.New("階段PDF在正式輸出前發生變動")}
 pdf,err:=parsePDF(name)
 if err!=nil{return "",fmt.Errorf("正式輸出檢查PDF失敗：%w",err)}
 if pdf.count!=state.Pages{return "",errors.New("正式PDF頁數與階段紀錄不符")}
 cleanup=false
 return name,nil
}

func publishCheckpointFile(folder,name,temporary string)(string,error){
 if filepath.Base(name)!=name||!strings.EqualFold(filepath.Ext(name),".pdf"){
  return "",errors.New("不安全的正式PDF檔名")
 }
 stem:=strings.TrimSuffix(name,filepath.Ext(name))
 for n:=1;n<=10000;n++{
  candidate:=name
  if n>1{candidate=fmt.Sprintf("%s (%d).pdf",stem,n)}
  output:=filepath.Join(folder,candidate)
  // A hard-link from the already completed temporary file is atomic and
  // exclusive: existing outputs cannot be overwritten or partially exposed.
  err:=linkCheckpointOutput(temporary,output)
  if err==nil{return output,nil}
  if os.IsExist(err){continue}
  return "",fmt.Errorf("無法安全發布正式PDF，請確認目標磁碟支援NTFS硬連結：%w",err)
 }
 return "",errors.New("輸出檔名編號已達上限")
}

func cleanupVerifiedStages(folder,stem string,state diagnosticCheckpoint)bool{
 retained:=false
 for n:=1;n<=state.Stage;n++{
  if len(state.StageSHA256)<n||state.StageSHA256[n-1]==""{retained=true;continue}
  path:=filepath.Join(folder,stagePDFName(stem,n))
  hash,err:=fingerprintFile(path)
  if os.IsNotExist(err){continue}
  if err!=nil||hash!=state.StageSHA256[n-1]{retained=true;continue}
  if err=os.Remove(path);err!=nil{retained=true}
 }
 return retained
}

// Called only by the user-initiated checkpoint action. Each call performs at
// most one checkpoint stage. No process relaunch or hidden continuation.
func runZIPPublishedStage(zipPath string,detailed bool)(diagnosticResult,error){
 stage,err:=runZIPDiagnosticStage(zipPath)
 if err!=nil||!stage.Finished{return stage,err}
 path,err:=filepath.Abs(zipPath)
 if err!=nil{return stage,err}
 folder,stem,statePath:=diagnosticNames(path)
 state,exists,err:=loadDiagnosticCheckpoint(statePath)
 if err!=nil{return stage,err}
 if !exists||!state.Finished{return stage,errors.New("缺少已完成的分段接續紀錄")}
 if state.FinalPDFPath!="" {
  stage.PDFPath=filepath.Join(folder,state.FinalPDFPath)
  stage.Published=true
  return stage,nil
 }
 job,err:=analyzeZIP(path)
 if err!=nil{return stage,err}
 name,fallback:=checkpointOutputName(job,detailed)
 temp,err:=verifiedStageCopy(folder,state)
 if err!=nil{return stage,err}
 defer os.Remove(temp)
 output,err:=publishCheckpointFile(folder,name,temp)
 if err!=nil{return stage,err}
 hash,err:=fingerprintFile(output)
 if err!=nil{return stage,err}
 if hash!=state.PDFSHA256{return stage,errors.New("正式PDF雜湊驗證失敗")}
 state.FinalPDFPath=filepath.Base(output)
 state.FinalPDFSHA256=hash
 state.UpdatedAt=time.Now().Format(time.RFC3339)
 if err=saveDiagnosticCheckpoint(statePath,state);err!=nil{
  // Do not delete the published PDF or previous stages when saving fails:
  // a valid final file has already been created for manual recovery.
  return stage,fmt.Errorf("正式PDF已輸出，但無法保存完成紀錄，請保留檔案：%w",err)
 }
 if cleanupVerifiedStages(folder,stem,state){stage.Warning="部分舊階段PDF尚未清理，請自行核對"}
 if fallback {
  if stage.Warning!=""{stage.Warning+="；"}
  stage.Warning+="來文機關或文號資料待核對"
 }
 stage.PDFPath=output
 stage.Published=true
 return stage,nil
}
