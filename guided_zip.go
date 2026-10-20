package main

// User-initiated staged ZIP merge.
// Each invocation processes at most two editable Office/ODF attachments,
// publishes one cumulative PDF next to the original ZIP, stores an explicit
// JSON checkpoint, and returns. A new user-launched process can then resume.
//
// The original v1.6.1 Office COM converter and PDF merge engine are unchanged.
// No subprocess relaunch, endpoint-protection control, or hidden background loop.
import (
 "archive/zip"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "strings"
 "time"
)

const guidedOfficePerStage=2
const guidedStateVersion=1

type guidedCheckpoint struct {
 Version int `json:"version"`
 SourceSHA256 string `json:"source_sha256"`
 Entries int `json:"entries"`
 Completed int `json:"completed"`
 Stage int `json:"stage"`
 PDFPath string `json:"pdf_path"`
 PDFSHA256 string `json:"pdf_sha256"`
 Pages int `json:"pages"`
 Finished bool `json:"finished"`
 FinalPDFPath string `json:"final_pdf,omitempty"`
 FinalPDFSHA256 string `json:"final_pdf_sha256,omitempty"`
 StageSHA256 []string `json:"stage_hashes,omitempty"`
 UpdatedAt string `json:"updated_at"`
}
type guidedResult struct {
 Stage int
 Converted int
 Completed int
 Total int
 Pages int
 PDFPath string
 CheckpointPath string
 Finished bool
 Duration time.Duration
 Published bool
 Warning string
}
func fingerprintFile(path string)(string,error){
 f,e:=os.Open(path)
 if e!=nil{return "",e}
 defer f.Close()
 sum:=sha256.New()
 if _,e=io.Copy(sum,f);e!=nil{return "",e}
 return hex.EncodeToString(sum.Sum(nil)),nil
}
func guidedZIPNames(path string)(folder,stem,statePath string){
 folder=filepath.Dir(path)
 stem=cleanComponent(strings.TrimSuffix(filepath.Base(path),filepath.Ext(path)))
 if stem==""{stem="公文ZIP"}
 statePath=filepath.Join(folder,stem+".手動分段.json")
 return
}
func guidedZIPStagePDFName(stem string,n int)string{
 return fmt.Sprintf("%s.手動分段第%03d階段.pdf",stem,n)
}
func loadGuidedCheckpoint(path string)(guidedCheckpoint,bool,error){
 var state guidedCheckpoint
 b,e:=os.ReadFile(path)
 if os.IsNotExist(e){return state,false,nil}
 if e!=nil{return state,false,e}
 if e=json.Unmarshal(b,&state);e!=nil{
  return state,false,fmt.Errorf("診斷紀錄損毀：%w",e)
 }
 return state,true,nil
}
func saveGuidedCheckpoint(path string,state guidedCheckpoint) error{
 b,e:=json.MarshalIndent(state,"","  ")
 if e!=nil{return e}
 f,e:=os.CreateTemp(filepath.Dir(path),".official-guided-state-*.tmp")
 if e!=nil{return e}
 tmp:=f.Name()
 defer os.Remove(tmp)
 if e=f.Chmod(0600);e!=nil{f.Close();return e}
 _,e=f.Write(append(b,'\n'))
 if e!=nil{f.Close();return e}
 if e=f.Sync();e!=nil{f.Close();return e}
 if e=f.Close();e!=nil{return e}
 // Replace only our own stable state file. It contains a ZIP hash and the
 // verified cumulative PDF hash and no actual document text.
 return os.Rename(tmp,path)
}
func guidedStageBoundary(entries []officialEntry,start int)(end,conversions int){
 end=start
 for end<len(entries){
  ext:=strings.ToLower(filepath.Ext(entries[end].name))
  if ext!=".pdf"&&conversions==guidedOfficePerStage{break}
  if ext!=".pdf"{conversions++}
  end++
 }
 return
}
func checkGuidedCheckpoint(state guidedCheckpoint,sourceHash string,total int,folder,stem string)error{
 if state.Version!=guidedStateVersion || state.Entries!=total || state.SourceSHA256!=sourceHash{
  return errors.New("來源ZIP與既有診斷紀錄不一致；請另用新檔名開始測試，不可直接續接")
 }
 if state.Completed<1||state.Completed>total||state.Stage<1{
  return errors.New("診斷紀錄的進度不合法")
 }
 if state.FinalPDFPath!="" {
  if !state.Finished || filepath.Base(state.FinalPDFPath)!=state.FinalPDFPath ||
   !strings.EqualFold(filepath.Ext(state.FinalPDFPath),".pdf") || state.FinalPDFSHA256=="" {
   return errors.New("已完成PDF紀錄異常")
  }
  final:=filepath.Join(folder,state.FinalPDFPath)
  hash,e:=fingerprintFile(final)
  if e!=nil{return fmt.Errorf("已完成PDF無法讀取：%w",e)}
  if hash!=state.FinalPDFSHA256{return errors.New("已完成PDF雜湊不符，停止續接")}
  p,e:=parsePDF(final)
  if e!=nil||p.count!=state.Pages{return errors.New("已完成PDF無法驗證")}
  return nil
 }
 expected:=guidedZIPStagePDFName(stem,state.Stage)
 if state.PDFPath!=expected{return errors.New("既有診斷紀錄的PDF路徑異常")}
 pdfPath:=filepath.Join(folder,expected)
 hash,e:=fingerprintFile(pdfPath)
 if e!=nil{return fmt.Errorf("前階段PDF遺失：%w",e)}
 if hash!=state.PDFSHA256{return errors.New("前階段PDF內容或雜湊已變更，拒絕接續")}
 parsed,e:=parsePDF(pdfPath)
 if e!=nil||parsed.count!=state.Pages{return errors.New("前階段PDF頁數或內容無法驗證")}
 if state.Finished&&state.Completed!=total{return errors.New("完成狀態與附件數量不符")}
 return nil
}
func mergeGuidedZIPRange(
 job *officialJob,begin,end int,priorPDF,out string,
)(int,error){
 zr,e:=zip.OpenReader(job.ZipPath)
 if e!=nil{return 0,e}
 defer zr.Close()
 workspace:=""
 defer func(){if workspace!=""{_ = os.RemoveAll(workspace)}}()
 getWorkspace:=func()(string,error){
  if workspace!=""{return workspace,nil}
  var err error
  workspace,err=createZIPWorkspace("","official-guided-*")
  return workspace,err
 }
 var docs []*pdfInput
 if priorPDF!=""{
  doc,err:=parsePDF(priorPDF)
  if err!=nil{return 0,fmt.Errorf("讀取前階段PDF失敗：%w",err)}
  docs=append(docs,doc)
 }
 var total,memoryPDF uint64
 for i:=begin;i<end;i++{
  entry:=job.Entries[i]
  if entry.index<0||entry.index>=len(zr.File){return 0,errors.New("ZIP附件索引異常")}
  item:=zr.File[entry.index]
  if item.UncompressedSize64>700*1024*1024{return 0,errors.New("附件超過大小限制")}
  if total>1200*1024*1024-item.UncompressedSize64{return 0,errors.New("附件總大小超出限制")}
  total+=item.UncompressedSize64
  ext:=strings.ToLower(filepath.Ext(entry.name))
  if ext==".pdf"&&item.UncompressedSize64<=zipPDFMemoryBudget-memoryPDF{
   bytes,err:=readZIPDocument(item)
   if err!=nil{return 0,err}
   doc,err:=parsePDFBytes(bytes,entry.name)
   if err!=nil{return 0,fmt.Errorf("%s：%w",entry.name,err)}
   docs=append(docs,doc)
   memoryPDF+=uint64(len(bytes))
   continue
  }
  folder,err:=getWorkspace()
  if err!=nil{return 0,err}
  input:=filepath.Join(folder,fmt.Sprintf("%04d%s",i,ext))
  if err=extractZIPDocument(item,input);err!=nil{return 0,fmt.Errorf("解壓%s失敗：%w",entry.name,err)}
  if ext!=".pdf"{
   converted:=filepath.Join(folder,fmt.Sprintf("%04d-converted.pdf",i))
   if err=convertEditable(input,converted);err!=nil{return 0,fmt.Errorf("%s轉檔失敗：%w",entry.name,err)}
   if err=verifyConvertedPDF(converted);err!=nil{return 0,fmt.Errorf("%s轉檔PDF無效：%w",entry.name,err)}
   input=converted
  }
  doc,err:=parsePDF(input)
  if err!=nil{return 0,fmt.Errorf("解析%s失敗：%w",entry.name,err)}
  docs=append(docs,doc)
 }
 if len(docs)<2 {
  // Stage tests focus on PDF merges (a main PDF plus attachments).
  return 0,errors.New("診斷階段至少需包含2份PDF；單一主文請使用原有功能")
 }
 return mergePDFDocuments(docs,out)
}

func runZIPGuidedStage(zipPath string)(guidedResult,error){
 var result guidedResult
 beginTime:=time.Now()
 path,e:=filepath.Abs(zipPath)
 if e!=nil{return result,e}
 info,e:=os.Stat(path)
 if e!=nil{return result,e}
 if !info.Mode().IsRegular(){return result,errors.New("診斷來源必須是一般ZIP檔")}
 if !strings.EqualFold(filepath.Ext(path),".zip"){return result,errors.New("診斷模式僅支援公文ZIP")}
 job,e:=analyzeZIP(path)
 if e!=nil{return result,e}
 if len(job.Entries)<2{return result,errors.New("診斷模式僅適用含附件的公文ZIP")}
 sha,e:=fingerprintFile(path)
 if e!=nil{return result,e}
 folder,stem,statePath:=guidedZIPNames(path)
 state,exists,e:=loadGuidedCheckpoint(statePath)
 if e!=nil{return result,e}
 if exists{
  if e=checkGuidedCheckpoint(state,sha,len(job.Entries),folder,stem);e!=nil{return result,e}
  if state.Finished {
   if state.FinalPDFPath!="" {
    return guidedResult{Stage:state.Stage,Completed:state.Completed,Total:state.Entries,
     Pages:state.Pages,PDFPath:filepath.Join(folder,state.FinalPDFPath),CheckpointPath:statePath,
     Finished:true,Published:true},nil
   }
   return guidedResult{
    Stage:state.Stage,Completed:state.Completed,Total:state.Entries,
    Pages:state.Pages,PDFPath:filepath.Join(folder,state.PDFPath),
    CheckpointPath:statePath,Finished:true,
   },nil
  }
 }else{
  state=guidedCheckpoint{Version:guidedStateVersion,SourceSHA256:sha,Entries:len(job.Entries)}
 }
 start:=state.Completed
 end,converted:=guidedStageBoundary(job.Entries,start)
 if end<=start{return result,errors.New("本階段沒有可處理的附件")}
 stage:=state.Stage+1
 name:=guidedZIPStagePDFName(stem,stage)
 out:=filepath.Join(folder,name)
 if _,err:=os.Stat(out);err==nil{return result,errors.New("階段輸出PDF已存在，不會覆寫："+name)} else if !os.IsNotExist(err){return result,err}
 prior:=""
 if exists{prior=filepath.Join(folder,state.PDFPath)}
 pages,e:=mergeGuidedZIPRange(job,start,end,prior,out)
 if e!=nil{return result,e}
 after,e:=fingerprintFile(path)
 if e!=nil||after!=sha{
  _=os.Remove(out)
  return result,errors.New("來源 ZIP 在合併過程中發生變動，已停止本階段")
 }
 // Only promote checkpoint once the complete new PDF exists and parses.
 verify,e:=parsePDF(out)
 if e!=nil||verify.count!=pages{
  _=os.Remove(out)
  return result,fmt.Errorf("階段結果PDF檢查失敗：%v",e)
 }
 pdfHash,e:=fingerprintFile(out)
 if e!=nil{_ =os.Remove(out);return result,e}
 history:=append([]string(nil),state.StageSHA256...)
 if len(history)!=state.Stage{
  history=make([]string,state.Stage)
  if state.Stage>0{history[state.Stage-1]=state.PDFSHA256}
 }
 history=append(history,pdfHash)
 next:=guidedCheckpoint{
  Version:guidedStateVersion,SourceSHA256:sha,Entries:len(job.Entries),
  Completed:end,Stage:stage,PDFPath:name,PDFSHA256:pdfHash,
  Pages:pages,Finished:end==len(job.Entries),StageSHA256:history,
  UpdatedAt:time.Now().Format(time.RFC3339),
 }
 if e=saveGuidedCheckpoint(statePath,next);e!=nil{
  _=os.Remove(out)
  return result,fmt.Errorf("無法記錄安全接續點：%w",e)
 }
 return guidedResult{
  Stage:stage,Converted:converted,Completed:end,Total:len(job.Entries),
  Pages:pages,PDFPath:out,CheckpointPath:statePath,
  Finished:next.Finished,Duration:time.Since(beginTime),
 },nil
}
