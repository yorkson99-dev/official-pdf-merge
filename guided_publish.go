package main

// Publication never overwrites a user's PDF. The final document is exposed
// atomically by linking the validated last stage on the same filesystem.
// No process relaunch, antivirus modifications or background continuation.
import (
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "time"
)

var guidedLinkFile = os.Link

func guidedFinalName(job *officialJob,detailed bool)(string,bool){
 name,e:=job.outputName(detailed)
 if e==nil{return name,false}
 fallback:=cleanComponent(strings.TrimSuffix(filepath.Base(job.ZipPath),filepath.Ext(job.ZipPath)))
 if fallback==""{fallback="來文"}
 return fallback+"_來文資料待確認.pdf",true
}

func guidedPublishState(folder,stem,statePath,targetName string,st guidedCheckpoint)(string,error){
 if !st.Finished||st.Completed!=st.Entries{return "",errors.New("文件尚未完成，不得正式輸出")}
 if st.FinalPDFPath!="" {
  if err:=checkGuidedCheckpoint(st,st.SourceSHA256,st.Entries,folder,stem);err!=nil{return "",err}
  return filepath.Join(folder,st.FinalPDFPath),nil
 }
 src:=filepath.Join(folder,st.PDFPath)
 hash,e:=fingerprintFile(src)
 if e!=nil{return "",e}
 if hash!=st.PDFSHA256{return "",errors.New("階段成果雜湊與紀錄不符")}
 p,e:=parsePDF(src)
 if e!=nil||p.count!=st.Pages{return "",errors.New("階段成果PDF頁數不符")}
 if filepath.Base(targetName)!=targetName||!strings.EqualFold(filepath.Ext(targetName),".pdf"){
  return "",errors.New("不安全的正式輸出檔名")
 }
 base:=strings.TrimSuffix(targetName,filepath.Ext(targetName))
 var dest string
 for n:=1;n<=10000;n++{
  name:=targetName
  if n>1{name=fmt.Sprintf("%s (%d).pdf",base,n)}
  candidate:=filepath.Join(folder,name)
  if e=guidedLinkFile(src,candidate);e==nil{dest=candidate;break}
  if !os.IsExist(e){return "",fmt.Errorf("正式輸出不可用（磁碟可能不支援原子建立連結）：%w",e)}
 }
 if dest==""{return "",errors.New("正式PDF檔名衝突過多")}
 hash,e=fingerprintFile(dest)
 if e!=nil||hash!=st.PDFSHA256{
  _=os.Remove(dest)
  return "",errors.New("正式PDF輸出校驗失敗")
 }
 st.FinalPDFPath=filepath.Base(dest)
 st.FinalPDFSHA256=hash
 st.UpdatedAt=time.Now().Format(time.RFC3339)
 if e=saveGuidedCheckpoint(statePath,st);e!=nil{
  // Do not delete already-published user data on a write failure.
  return "",fmt.Errorf("已產生正式PDF %s，但完成紀錄無法寫入：%w",filepath.Base(dest),e)
 }
 // Remove only known stage artifacts with exactly matching expected hashes.
 for i:=1;i<=st.Stage && i<=len(st.StageSHA256);i++{
  stageHash:=st.StageSHA256[i-1]
  if stageHash==""{continue}
  file:=filepath.Join(folder,guidedZIPStagePDFName(stem,i))
  current,e:=fingerprintFile(file)
  if e==nil&&current==stageHash{_ =os.Remove(file)}
 }
 return dest,nil
}

func runZIPGuidedPublishedStage(path string,detailed bool)(guidedResult,error){
 outcome,e:=runZIPGuidedStage(path)
 if e!=nil||!outcome.Finished{return outcome,e}
 path,e=filepath.Abs(path)
 if e!=nil{return outcome,e}
 folder,stem,statePath:=guidedZIPNames(path)
 state,ok,e:=loadGuidedCheckpoint(statePath)
 if e!=nil{return outcome,e}
 if !ok{return outcome,errors.New("找不到可驗證的手動分段接續紀錄")}
 if state.FinalPDFPath!=""{
  outcome.PDFPath=filepath.Join(folder,state.FinalPDFPath)
  outcome.Published=true
  return outcome,nil
 }
 job,e:=analyzeZIP(path)
 if e!=nil{return outcome,e}
 name,fallback:=guidedFinalName(job,detailed)
 dest,e:=guidedPublishState(folder,stem,statePath,name,state)
 if e!=nil{return outcome,e}
 outcome.PDFPath=dest
 outcome.Published=true
 if fallback{outcome.Warning="來文資訊未完整辨識，請核對檔名"}
 return outcome,nil
}
