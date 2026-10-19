package main

import "testing"

func TestMixedWarningPreflightRules(t *testing.T) {
    cases:=[]struct{name string; paths []string; warning bool}{
        {"single editable",[]string{"a.odt"},false},
        {"three with Office",[]string{"a.pdf","b.docx","c.odt"},false},
        {"three PDFs",[]string{"1.pdf","2.pdf","3.pdf"},false},
        {"four PDFs",[]string{"1.pdf","2.pdf","3.pdf","4.pdf"},false},
        {"thirteen PDFs",[]string{
            "1.pdf","2.pdf","3.pdf","4.pdf","5.pdf","6.pdf","7.pdf",
            "8.pdf","9.pdf","10.pdf","11.pdf","12.pdf","13.pdf"},false},
        {"one ODT and three PDFs",[]string{"a.pdf","b.pdf","c.pdf","d.odt"},false},
        {"one DOCX and three PDFs",[]string{"a.pdf","b.docx","c.pdf","d.pdf"},false},
        {"one ODS and three PDFs",[]string{"a.ods","b.pdf","c.pdf","d.pdf"},false},
        {"uppercase PDF recognized",[]string{"A.PDF","B.PDF","C.PDF","D.PdF"},false},
        {"mixed extensions case insensitive",[]string{"A.PDF","B.DoCx","C.ODT","D.ODS"},true},
        {"five Office-only documents",[]string{"a.docx","b.odt","c.ods","d.pptx","e.txt"},true},
        {"exactly two Office conversions with many PDFs",[]string{"a.docx","b.odt","1.pdf","2.pdf","3.pdf","4.pdf","5.pdf","6.pdf","7.pdf","8.pdf","9.pdf","10.pdf","11.pdf","12.pdf","13.pdf"},false},
        {"three Office conversions",[]string{"a.docx","b.odt","c.ods"},true},
    }
    for _,tc:=range cases{
        t.Run(tc.name,func(t *testing.T){
            got:=needsMixedFileWarning(tc.paths)
            if got!=tc.warning{t.Fatalf("warning=%t, want %t",got,tc.warning)}
        })
    }
}

func TestZIPMixedWarningCountsOnlyMergeEntries(t *testing.T) {
    selected:=[]officialEntry{
        {name:"letter.pdf"},
        {name:"letter_ATTCH1.pdf",attachment:true,number:1},
        {name:"letter_ATTCH2.odt",attachment:true,number:2},
    }
    if needsMixedZIPWarning(selected){t.Fatal("three selected documents should not warn")}
    // ZIP may also contain an opinion sheet, drafts and excluded files,
    // but only officialJob.Entries must count toward the threshold.
    selected=append(selected,officialEntry{name:"letter_ATTCH3.docx",attachment:true,number:3})
    if needsMixedZIPWarning(selected){t.Fatal("two Office documents must not warn")}
    selected=append(selected,officialEntry{name:"letter_ATTCH4.ods",attachment:true,number:4})
    if !needsMixedZIPWarning(selected){t.Fatal("three Office documents must warn")}
    selected[2].name="letter_ATTCH2.pdf"
    if needsMixedZIPWarning(selected){t.Fatal("four selected PDFs must never warn")}
}
