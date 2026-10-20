package main

// Manually confirmed ordinary-file batches reuse the PDF/Office conversion
// engine. The same ordered input list must be provided for every stage.
// Receipt files contain fingerprints only, never document bytes or names.
import (
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "time"
)

func guidedGeneralReceiptPath(inputs []string)(string,error){
 if len(inputs)==0{return "",errors.New("請先選擇文件")}
 paths:=make([]string,0,len(inputs))
 for _,path:=range inputs{
  abs,e:=filepath.Abs(path)
  if e!=nil{return "",e}
  paths=append(paths,strings.ToLower(abs))
 }
 // Sort only for identity: the actual merge order remains user-controlled.
 // A changed order must reach the same receipt and fail its source checksum.
 sort.Strings(paths)
 sha:=sha256.New()
 for _,path:=range paths{_,_=io.WriteString(sha,path+"\\x00")}
 identity:=hex.EncodeToString(sha.Sum(nil))
 first,e:=filepath.Abs(inputs[0])
 if e!=nil{return "",e}
 folder:=filepath.Dir(first)
 return filepath.Join(folder,"合併檔案-"+identity[:12]+".手動分段.json"),nil
}

func guidedGeneralIdentity(inputs []string)(folder,stem,sourceHash,statePath string,err error){
 statePath,err=guidedGeneralReceiptPath(inputs)
 if err!=nil{return}
 folder=filepath.Dir(statePath)
 stem=strings.TrimSuffix(filepath.Base(statePath),".手動分段.json")
 content:=sha256.New()
 for _,name:=range inputs{
  abs,e:=filepath.Abs(name)
  if e!=nil{err=e;return}
  info,e:=os.Stat(abs)
  if e!=nil{err=e;return}
  if !info.Mode().IsRegular(){err=errors.New("包含非一般檔案");return}
  if info.Size()>700*1024*1024{err=errors.New("來源文件超過大小限制");return}
  if !supportedOfficialExt(filepath.Ext(abs)){err=errors.New("包含不支援的文件");return}
  fp,e:=fingerprintFile(abs)
  if e!=nil{err=e;return}
  _,_=io.WriteString(content,strings.ToLower(abs)+"\\x00"+fp+"\\n")
 }
 sourceHash=hex.EncodeToString(content.Sum(nil))
 return
}

func runGeneralGuidedStage(inputs []string)(guidedResult,error){
 var res guidedResult
 startTime:=time.Now()
 folder,stem,hash,statePath,e:=guidedGeneralIdentity(inputs)
 if e!=nil{return res,e}
 state,exists,e:=loadGuidedCheckpoint(statePath)
 if e!=nil{return res,e}
 if exists{
  if e=checkGuidedCheckpoint(state,hash,len(inputs),folder,stem);e!=nil{return res,e}
  if state.Finished{
   name:=state.PDFPath
   if state.FinalPDFPath!=""{name=state.FinalPDFPath}
   res=guidedResult{Stage:state.Stage,Completed:state.Completed,Total:state.Entries,
    Pages:state.Pages,PDFPath:filepath.Join(folder,name),CheckpointPath:statePath,
    Finished:true,Published:state.FinalPDFPath!=""}
  }
 }else{
  state=guidedCheckpoint{Version:guidedStateVersion,SourceSHA256:hash,Entries:len(inputs)}
 }
 if !state.Finished{
  start:=state.Completed
  entries:=make([]officialEntry,len(inputs))
  for i,p:=range inputs{entries[i]=officialEntry{name:p}}
  end,converted:=guidedStageBoundary(entries,start)
  if end<=start{return res,errors.New("下一階段沒有文件")}
  stage:=state.Stage+1
  filename:=guidedZIPStagePDFName(stem,stage)
  out:=filepath.Join(folder,filename)
  if _,e=os.Stat(out);e==nil{return res,errors.New("已有同名階段PDF，拒絕覆寫")}
  if !os.IsNotExist(e){return res,e}
  batch:=make([]string,0,1+end-start)
  if exists{batch=append(batch,filepath.Join(folder,state.PDFPath))}
  batch=append(batch,inputs[start:end]...)
  pages,e:=mergeMixedFiles(batch,out)
  if e!=nil{return res,e}
  _,_,afterHash,_,e:=guidedGeneralIdentity(inputs)
  if e!=nil||afterHash!=hash{
   _=os.Remove(out)
   return res,errors.New("來源文件在合併過程中發生變動，已停止本階段")
  }
  pdf,e:=parsePDF(out)
  if e!=nil||pdf.count!=pages{_ =os.Remove(out);return res,errors.New("階段PDF驗證失敗")}
  sum,e:=fingerprintFile(out)
  if e!=nil{_ =os.Remove(out);return res,e}
  history:=append(append([]string(nil),state.StageSHA256...),sum)
  next:=guidedCheckpoint{Version:guidedStateVersion,SourceSHA256:hash,Entries:len(inputs),
   Completed:end,Stage:stage,PDFPath:filename,PDFSHA256:sum,Pages:pages,
   Finished:end==len(inputs),StageSHA256:history,UpdatedAt:time.Now().Format(time.RFC3339)}
  if e=saveGuidedCheckpoint(statePath,next);e!=nil{_ =os.Remove(out);return res,e}
  state=next
  res=guidedResult{Stage:stage,Converted:converted,Completed:end,Total:len(inputs),
   Pages:pages,PDFPath:out,CheckpointPath:statePath,Finished:next.Finished,Duration:time.Since(startTime)}
 }
 if !state.Finished{return res,nil}
 if state.FinalPDFPath!=""{res.Published=true;return res,nil}
 name:="合併檔案.pdf"
 dest,e:=guidedPublishState(folder,stem,statePath,name,state)
 if e!=nil{return res,fmt.Errorf("正式PDF發布失敗，接續狀態已保存：%w",e)}
 res.PDFPath=dest
 res.Published=true
 return res,nil
}
