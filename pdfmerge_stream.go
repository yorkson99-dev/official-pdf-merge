package main

// Streaming PDF writer: only one input PDF is parsed at a time. The destination
// is one temporary sibling file, atomically published after all pages are valid.
// No per-stage merged PDF is ever created.
import (
    "bufio"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
)

type pdfStreamMerge struct {
    output string
    temp *os.File
    tempName string
    buffer *bufio.Writer
    writer *pdfWriter
    pageRoots []string
    pages int
    published bool
}

func newPDFStreamMerge(output string) (*pdfStreamMerge, error) {
    if _,err:=os.Stat(output);err==nil{
        return nil,errors.New("輸出檔已存在，請更換檔名")
    } else if !os.IsNotExist(err) {return nil,err}
    f,err:=os.CreateTemp(filepath.Dir(output),".fastpdfmerge-*.tmp")
    if err!=nil{return nil,err}
    b:=bufio.NewWriterSize(f,256*1024)
    s:=&pdfStreamMerge{
        output:output,temp:f,tempName:f.Name(),buffer:b,
        writer:&pdfWriter{out:b,offsets:make([]int64,3)},
    }
    if err=s.writer.write([]byte("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n"));err!=nil{
        s.abort()
        return nil,err
    }
    // PDF object 2 (the shared Pages node) is written only after the last
    // document. Its location is recorded in the final cross-reference table.
    if err=s.writer.object(1,[]byte("<< /Type /Catalog /Pages 2 0 R >>"));err!=nil{
        s.abort()
        return nil,err
    }
    return s,nil
}

func (s *pdfStreamMerge) appendPDF(doc *pdfInput) error {
    if doc==nil || doc.count<1{return errors.New("無效的 PDF 輸入")}
    if s.pages+doc.count>100000{return errors.New("總頁數超出 100,000 頁上限")}
    keys:=make([]objKey,0,len(doc.objects))
    for key,obj:=range doc.objects{
        dict:=objectDict(obj.data)
        if typeObjStmPattern.Match(dict)||typeXrefPattern.Match(dict){continue}
        keys=append(keys,key)
    }
    sort.Slice(keys,func(i,j int)bool{
        if keys[i].id!=keys[j].id{return keys[i].id<keys[j].id}
        return keys[i].generation<keys[j].generation
    })
    start:=len(s.writer.offsets)
    mapping:=make(map[objKey]int,len(keys))
    for i,key:=range keys{mapping[key]=start+i}
    root,ok:=mapping[doc.root]
    if !ok{return errors.New("找不到頁面樹根節點")}
    s.writer.offsets=append(s.writer.offsets,make([]int64,len(keys))...)
    // Write this document's relocated objects immediately. The caller may
    // release its entire parsed PDF before loading the next attachment.
    for _,key:=range keys{
        body:=relocateObject(doc.objects[key],mapping)
        if key==doc.root{
            var err error
            body,err=appendParent(body,2)
            if err!=nil{return err}
        }
        if err:=s.writer.object(mapping[key],body);err!=nil{return err}
    }
    s.pages+=doc.count
    s.pageRoots=append(s.pageRoots,fmt.Sprintf("%d 0 R",root))
    return nil
}

func (s *pdfStreamMerge) commit() (int,error) {
    if len(s.pageRoots)<2 {return 0,errors.New("至少需要2份 PDF")}
    pageNode:=[]byte(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [ %s ] >>",s.pages,strings.Join(s.pageRoots," ")))
    if err:=s.writer.object(2,pageNode);err!=nil{return 0,err}
    if err:=s.writer.finish(1);err!=nil{return 0,err}
    if err:=s.buffer.Flush();err!=nil{return 0,err}
    if err:=s.temp.Sync();err!=nil{return 0,err}
    if err:=s.temp.Close();err!=nil{return 0,err}
    if _,err:=os.Stat(s.output);err==nil{
        return 0,errors.New("輸出檔已存在，請更換檔名")
    } else if !os.IsNotExist(err){return 0,err}
    if err:=os.Rename(s.tempName,s.output);err!=nil{return 0,err}
    s.published=true
    return s.pages,nil
}

func (s *pdfStreamMerge) abort() {
    if s==nil{return}
    if s.temp!=nil {_=s.temp.Close()}
    if !s.published&&s.tempName!="" {_=os.Remove(s.tempName)}
}
