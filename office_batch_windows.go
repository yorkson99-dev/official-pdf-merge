//go:build windows

package main

// Reuse one Word/Excel/PowerPoint instance for consecutive files of the same
// type. Conversion is strictly serial on one COM STA, not parallel.
// Preserve the original Office export methods and security options.
import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "runtime"
    "strings"
)

type officeBatchSession struct {
    kind string
    app, files *oleDispatch
}

func (s *officeBatchSession) close() {
    if s.files!=nil {
        s.files.release()
        s.files=nil
    }
    if s.app!=nil {
        if s.kind=="word" {
            _=s.app.method("Quit",oleInteger(0))
        } else {
            _=s.app.method("Quit")
        }
        s.app.release()
        s.app=nil
    }
    s.kind=""
}

func (s *officeBatchSession) start(kind string) error {
    if s.kind==kind && s.app!=nil && s.files!=nil{return nil}
    s.close()
    prog:=map[string]string{"word":"Word.Application","excel":"Excel.Application","powerpoint":"PowerPoint.Application"}[kind]
    if prog==""{return fmt.Errorf("不支援的 Office 類型：%s",kind)}
    app,err:=oleStart(prog)
    if err!=nil{return err}
    s.app=app
    s.kind=kind
    switch kind {
    case "word":
        if err=app.put("Visible",oleBoolean(false));err!=nil{return err}
        if err=app.put("DisplayAlerts",oleInteger(0));err!=nil{return err}
        if err=app.put("AutomationSecurity",oleInteger(3));err!=nil{return fmt.Errorf("未確認巨集已停用：%w",err)}
        s.files,err=app.getObject("Documents")
    case "excel":
        for _,entry:=range []struct{name string;v oleVariant}{
            {"Visible",oleBoolean(false)},{"DisplayAlerts",oleBoolean(false)},
            {"AskToUpdateLinks",oleBoolean(false)},{"AutomationSecurity",oleInteger(3)},
        } {
            if err=app.put(entry.name,entry.v);err!=nil{return fmt.Errorf("Excel 設定 %s：%w",entry.name,err)}
        }
        s.files,err=app.getObject("Workbooks")
    case "powerpoint":
        if err=app.put("AutomationSecurity",oleInteger(3));err!=nil{return fmt.Errorf("未確認巨集已停用：%w",err)}
        s.files,err=app.getObject("Presentations")
    }
    return err
}

func (s *officeBatchSession) convert(input,output string) error {
    switch s.kind {
    case "word":
        doc,err:=s.files.textObjectMethod("Open",input,oleBoolean(false),oleBoolean(true),oleBoolean(false))
        if err!=nil{return err}
        defer doc.release()
        defer func(){_=doc.method("Close",oleInteger(0))}()
        if err=doc.textMethod("ExportAsFixedFormat",output,oleInteger(17));err!=nil{return err}
    case "excel":
        book,err:=s.files.textObjectMethod("Open",input,oleInteger(0),oleBoolean(true))
        if err!=nil{return err}
        defer book.release()
        defer func(){_=book.method("Close",oleBoolean(false))}()
        file,err:=oleBSTR(output)
        if err!=nil{return err}
        if err=book.method("ExportAsFixedFormat",oleInteger(0),file);err!=nil{return err}
    case "powerpoint":
        pres,err:=s.files.textObjectMethod("Open",input,oleInteger(-1),oleInteger(0),oleInteger(0))
        if err!=nil{return err}
        defer pres.release()
        defer func(){_=pres.method("Close") }()
        if err=pres.textMethod("SaveAs",output,oleInteger(32));err!=nil{return err}
    default:
        return fmt.Errorf("不支援的 Office 類型：%s",s.kind)
    }
    return verifyConvertedPDF(output)
}

// convertOfficeCOMBatch never starts concurrent Office applications for a
// document sequence. When the type changes, close the previous application
// before moving on. The final PDF merge still happens only once.
func convertOfficeCOMBatch(inputs,outputs []string) error {
    if len(inputs)==0 || len(inputs)!=len(outputs){return errors.New("批次轉檔來源與輸出數量不符")}
    for i,src:=range inputs {
        ext:=strings.ToLower(filepath.Ext(src))
        if !supportedOfficialExt(ext)||ext==".pdf"{return fmt.Errorf("不支援的文件：%s",filepath.Base(src))}
        st,err:=os.Stat(src)
        if err!=nil{return fmt.Errorf("無法讀取 %s：%w",filepath.Base(src),err)}
        if st.IsDir(){return fmt.Errorf("來源不是檔案：%s",filepath.Base(src))}
        if _,err:=os.Stat(outputs[i]);err==nil{return fmt.Errorf("輸出檔已存在：%s",filepath.Base(outputs[i]))} else if !os.IsNotExist(err) {return err}
    }

    runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    hr,_,_:=coInitializeExProc.Call(0,coinitApartmentThreaded)
    if oleFailed(hr){return fmt.Errorf("初始化 COM 失敗：%w",oleHResult(hr))}
    defer coUninitializeProc.Call()
    session:=&officeBatchSession{}
    defer session.close()

    for i,input:=range inputs {
        kind,err:=officeKind(filepath.Ext(input))
        if err!=nil{return err}
        if err=session.start(kind);err!=nil{
            return fmt.Errorf("啟動 %s 轉檔引擎失敗：%w",filepath.Base(input),err)
        }
        if err=session.convert(input,outputs[i]);err!=nil{
            return fmt.Errorf("轉換 %s 失敗（第%d/%d份）：%w",filepath.Base(input),i+1,len(inputs),err)
        }
    }
    return nil
}
