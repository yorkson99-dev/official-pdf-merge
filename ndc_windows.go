//go:build windows

package main

// NDC ODF Application Tools (LibreOffice-based) PDF converter.
// Uses only the installed NDC binary; no Office COM, PowerShell or network.
import (
    "context"
    "errors"
    "fmt"
    "io"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "syscall"
    "time"
)

const ndcDefaultBinary = `C:\Program Files\NDC ODF Application Tools\program\soffice.com`

// Test seams verify command arguments and exported files without depending on
// NDC being installed on the GitHub-hosted Windows runner.
var ndcLocateBinary = locateNDCBinary
var ndcExecute = runNDCProcess

func locateNDCBinary() (string, error) {
    candidates := []string{ndcDefaultBinary}
    for _, env := range []string{"ProgramW6432", "ProgramFiles"} {
        if folder := os.Getenv(env); folder != "" {
            candidates = append(candidates, filepath.Join(folder, "NDC ODF Application Tools", "program", "soffice.com"))
        }
    }
    for _, p := range candidates {
        st, err := os.Stat(p)
        if err == nil && !st.IsDir() { return p, nil }
    }
    return "", errors.New("找不到 NDC ODF 的 soffice.com。請確認已安裝 NDC ODF 3.3.3（x64）於 C:\\Program Files\\NDC ODF Application Tools\\program")
}

func ndcProfileURI(profile string) (string, error) {
    abs, err := filepath.Abs(profile)
    if err != nil { return "", err }
    slash := filepath.ToSlash(abs)
    if len(slash) < 3 || slash[1] != ':' { return "", errors.New("NDC ODF 設定檔必須使用 Windows 本機磁碟路徑") }
    u := url.URL{Scheme:"file",Path:"/"+slash}
    return u.String(), nil
}

func ndcArgs(profile, outDir string, inputs []string) ([]string,error) {
    uri,err := ndcProfileURI(profile)
    if err!=nil{return nil,err}
    args := []string{"-env:UserInstallation="+uri,"--headless","--nologo","--nodefault","--nofirststartwizard","--convert-to","pdf","--outdir",outDir}
    args=append(args, inputs...)
    return args,nil
}

func runNDCProcess(ctx context.Context, binary string, args []string) (string,error) {
    cmd:=exec.CommandContext(ctx,binary,args...)
    // Hide the console for normal GUI use; this is not a security bypass.
    cmd.SysProcAttr=&syscall.SysProcAttr{HideWindow:true}
    result,err:=cmd.CombinedOutput()
    // Avoid returning excessively long output containing document metadata.
    message:=strings.TrimSpace(string(result))
    if len(message)>1200 {message=message[:1200]}
    return message,err
}

func ndcCopyFile(source,dest string) error {
    in,err:=os.Open(source)
    if err!=nil{return err}
    defer in.Close()
    out,err:=os.OpenFile(dest,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
    if err!=nil{return err}
    _,copyErr:=io.Copy(out,in)
    if copyErr==nil{copyErr=out.Sync()}
    if closeErr:=out.Close();copyErr==nil{copyErr=closeErr}
    if copyErr!=nil{os.Remove(dest)}
    return copyErr
}

// NDC ODF produces <source basename>.pdf in --outdir. Stage only inputs whose
// basenames collide in one batch; otherwise pass their original paths directly.
func ndcUniqueInputs(inputs []string, staging string) ([]string,error) {
    count:=make(map[string]int,len(inputs))
    for _,input:=range inputs {
        stem:=strings.ToLower(strings.TrimSuffix(filepath.Base(input),filepath.Ext(input)))
        count[stem]++
    }
    unique:=make([]string,len(inputs))
    for i,input:=range inputs {
        stem:=strings.ToLower(strings.TrimSuffix(filepath.Base(input),filepath.Ext(input)))
        if count[stem]==1 {unique[i]=input;continue}
        path:=filepath.Join(staging,fmt.Sprintf("ndc-%04d%s",i,filepath.Ext(input)))
        if err:=ndcCopyFile(input,path);err!=nil{return nil,err}
        unique[i]=path
    }
    return unique,nil
}

// Convert several Office/ODF files in one NDC process, while preserving the
// original order in caller-supplied output paths. Each conversion job gets an
// isolated profile to avoid sharing the user's NDC session. This does not
// modify NDC's installed files, security policies, or the user's profile.
func convertDocumentsToPDFBatch(inputs,outputs []string) error {
    if len(inputs)==0 || len(inputs)!=len(outputs) {
        return errors.New("轉檔來源與目的檔案數量不符")
    }
    exe,err:=ndcLocateBinary()
    if err!=nil{return err}
    for i,in:=range inputs {
        ext:=strings.ToLower(filepath.Ext(in))
        if ext==".pdf" || !supportedOfficialExt(ext) {
            return fmt.Errorf("不支援的轉檔格式：%s",filepath.Base(in))
        }
        if st,e:=os.Stat(in);e!=nil || st.IsDir() {
            return fmt.Errorf("找不到轉檔來源：%s",filepath.Base(in))
        }
        if _,e:=os.Stat(outputs[i]);e==nil {
            return fmt.Errorf("輸出檔已存在：%s",filepath.Base(outputs[i]))
        } else if !os.IsNotExist(e) {
            return e
        }
    }
    work,err:=os.MkdirTemp("","official-pdf-ndc-*")
    if err!=nil{return err}
    defer os.RemoveAll(work)
    profile:=filepath.Join(work,"profile")
    converted:=filepath.Join(work,"converted")
    staged:=filepath.Join(work,"staged")
    for _,folder:=range []string{profile,converted,staged} {
        if err:=os.Mkdir(folder,0700);err!=nil{return err}
    }
    uniques,err:=ndcUniqueInputs(inputs,staged)
    if err!=nil{return fmt.Errorf("準備 NDC ODF 轉檔資料失敗：%w",err)}
    args,err:=ndcArgs(profile,converted,uniques)
    if err!=nil{return err}
    limit:=2*time.Minute+time.Duration(len(inputs))*time.Minute
    if limit>12*time.Minute{limit=12*time.Minute}
    ctx,cancel:=context.WithTimeout(context.Background(),limit)
    defer cancel()
    message,err:=ndcExecute(ctx,exe,args)
    if ctx.Err()==context.DeadlineExceeded {
        return fmt.Errorf("NDC ODF 轉檔逾時（%s），請確認是否有文件開啟提示或設定檔問題",limit)
    }
    if err!=nil {return fmt.Errorf("NDC ODF 轉檔失敗：%w；%s",err,message)}
    for i,unique:=range uniques {
        stem:=strings.TrimSuffix(filepath.Base(unique),filepath.Ext(unique))
        pdf:=filepath.Join(converted,stem+".pdf")
        if err:=verifyConvertedPDF(pdf);err!=nil {
            return fmt.Errorf("NDC ODF 未成功轉換 %s：%w；%s",filepath.Base(inputs[i]),err,message)
        }
    }
    // Validate every generated PDF before publishing any to the caller.
    // Caller will suppress final merged output if any conversion fails.
    for i,unique:=range uniques {
        stem:=strings.TrimSuffix(filepath.Base(unique),filepath.Ext(unique))
        pdf:=filepath.Join(converted,stem+".pdf")
        if err:=ndcCopyFile(pdf,outputs[i]);err!=nil{
            for k:=0;k<i;k++ {os.Remove(outputs[k])}
            return fmt.Errorf("儲存 %s 的轉檔結果失敗：%w",filepath.Base(inputs[i]),err)
        }
    }
    return nil
}
