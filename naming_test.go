package main

import "testing"

func TestDetailedNameAddsAttachmentSuffix(t *testing.T) {
    j := &officialJob{
        Agency: "臺灣士林地方法院",
        Date: "115年10月12日",
        DocumentNo: "士院璿民常115訴1073字第1159028285號",
        Kind: "函",
        Entries: []officialEntry{{name:"letter.pdf"}},
    }
    name, err := j.outputName(true)
    if err != nil { t.Fatal(err) }
    want := "臺灣士林地方法院115年10月12日士院璿民常115訴1073字第1159028285號函.pdf"
    if name != want { t.Fatalf("without attachments got %q", name) }
    j.Entries = append(j.Entries, officialEntry{name:"letter_ATTCH1.odt",attachment:true,number:1})
    name, err = j.outputName(true)
    if err != nil {t.Fatal(err)}
    want = "臺灣士林地方法院115年10月12日士院璿民常115訴1073字第1159028285號函及其附件.pdf"
    if name != want {t.Fatalf("with attachments got %q",name)}
    simple,err:=j.outputName(false)
    if err!=nil || simple!="臺灣士林地方法院公文及其附件.pdf" {t.Fatalf("simple %q: %v",simple,err)}
}
