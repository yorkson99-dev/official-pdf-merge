package main

// Approval-folder ZIP selections. This does not change the original 來文
// parser. ZIP member names can be UTF-8 or raw CP950 bytes.
import (
 "archive/zip"
 "errors"
 "fmt"
 "path/filepath"
 "regexp"
 "sort"
 "strings"
)

type zipMergeMode string
const (
 zipModeLetter zipMergeMode = "來文"
 zipModeSign zipMergeMode = "簽"
 zipModeDraft zipMergeMode = "稿"
)

const (
 zipSignCP950 = "\xc3\xb1"
 zipDraftCP950 = "\xbd\x5a"
)

var zipDocumentID = regexp.MustCompile("[0-9]{8,}$")

func zipRootMode(name string) zipMergeMode {
 path:=strings.ReplaceAll(name,"\\","/")
 // Leading empty path segments and traversal-style names never qualify.
 parts:=strings.SplitN(path,"/",2)
 if len(parts)!=2 || parts[0]=="" || parts[1]=="" {return ""}
 switch parts[0] {
 case "來文","\xa8\xd3\xa4\xe5":return zipModeLetter
 case "簽",zipSignCP950:return zipModeSign
 case "稿",zipDraftCP950:return zipModeDraft
 }
 return ""
}

func availableZIPModes(path string)([]zipMergeMode,error){
 zr,e:=zip.OpenReader(path)
 if e!=nil{return nil,fmt.Errorf("ZIP 無法開啟：%w",e)}
 defer zr.Close()
 if len(zr.File)>4000{return nil,errors.New("ZIP 檔案項目過多")}
 found:=map[zipMergeMode]bool{}
 for _,f:=range zr.File {
  mode:=zipRootMode(f.Name)
  if mode!="" {found[mode]=true}
 }
 modes:=make([]zipMergeMode,0,3)
 for _,mode:=range []zipMergeMode{zipModeLetter,zipModeSign,zipModeDraft}{
  if found[mode]{modes=append(modes,mode)}
 }
 // Retain the legacy 來文-specific error text for unrelated ZIPs.
 if len(modes)==0 {return []zipMergeMode{zipModeLetter},nil}
 return modes,nil
}

func modePDFName(mode zipMergeMode)(string,error){
 switch mode{
 case zipModeSign:return "奉核簽.pdf",nil
 case zipModeDraft:return "奉核稿.pdf",nil
 default:return "",fmt.Errorf("不支援的合併類型：%s",mode)
 }
}

func matchesCaseAfter(base,caseID string)bool{
 if caseID=="" {return false}
 lower:=strings.ToLower(base)
 if !strings.HasSuffix(lower,"(after).pdf"){return false}
 stem:=base[:len(base)-len("(after).pdf")]
 return stem==caseID || strings.HasPrefix(stem,caseID+"_")
}

// Only directly nested files in 簽 or 稿 count. Any deeper 參考附件 or
// other folder is excluded completely. After-PDF must match the case number
// found at the end of the archive basename.
func analyzeApprovalZIP(path string, mode zipMergeMode)(*officialJob,error){
 if mode!=zipModeSign && mode!=zipModeDraft{return nil,errors.New("不支援的奉核合併類型")}
 stem:=strings.TrimSuffix(filepath.Base(path),filepath.Ext(path))
 caseID:=zipDocumentID.FindString(stem)
 if caseID==""{return nil,errors.New("ZIP 檔名結尾找不到公文號，無法確認 (after) 主文")}
 zr,e:=zip.OpenReader(path)
 if e!=nil{return nil,fmt.Errorf("ZIP 無法開啟：%w",e)}
 defer zr.Close()
 if len(zr.File)>4000{return nil,errors.New("ZIP 檔案項目過多")}
 var main *officialEntry
 attachments:=[]officialEntry{}
 seenFolder:=false
 for i,f:=range zr.File{
  if zipRootMode(f.Name)!=mode{continue}
  seenFolder=true
  if f.FileInfo().IsDir(){continue}
  pathInZip:=strings.ReplaceAll(f.Name,"\\","/")
  parts:=strings.SplitN(pathInZip,"/",2)
  if len(parts)!=2 || parts[1]=="" || strings.Contains(parts[1],"/"){continue}
  rawBase:=parts[1]
  base:=displayZIPFileName(rawBase)
  ext:=strings.ToLower(filepath.Ext(rawBase))
  if f.UncompressedSize64>700*1024*1024{return nil,fmt.Errorf("%s 內容過大",base)}
  if !supportedOfficialExt(ext){return nil,fmt.Errorf("%s 資料夾有不支援的直接子檔案：%s",mode,base)}
  entry:=officialEntry{index:i,name:base,group:string(mode)}
  if strings.HasSuffix(strings.ToLower(rawBase),"(after).pdf"){
   if !matchesCaseAfter(rawBase,caseID){
    return nil,fmt.Errorf("%s 資料夾有不同公文號的 (after) PDF：%s",mode,base)
   }
   if main!=nil{return nil,fmt.Errorf("%s 資料夾有多份符合公文號的 (after) 主文，無法決定順序",mode)}
   saved:=entry
   main=&saved
  }else{
   entry.attachment=true
   attachments=append(attachments,entry)
  }
 }
 if !seenFolder{return nil,fmt.Errorf("ZIP 找不到「%s」資料夾",mode)}
 if main==nil{return nil,fmt.Errorf("%s 資料夾找不到以 %s 開頭且以 (after).pdf 結尾的主文",mode,caseID)}
 sort.SliceStable(attachments,func(i,j int)bool{
  left:=strings.ToLower(attachments[i].name)
  right:=strings.ToLower(attachments[j].name)
  if left==right{return attachments[i].index<attachments[j].index}
  return left<right
 })
 entries:=make([]officialEntry,0,1+len(attachments))
 entries=append(entries,*main)
 entries=append(entries,attachments...)
 return &officialJob{ZipPath:path,Entries:entries,Kind:string(mode)},nil
}

func analyzeZIPMode(path string,mode zipMergeMode)(*officialJob,error){
 if mode==zipModeLetter{return analyzeZIP(path)}
 return analyzeApprovalZIP(path,mode)
}
