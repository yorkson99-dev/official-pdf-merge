package main

// Synthetic fixtures only: never commit user-provided government ZIP contents.
import (
 "archive/zip"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

type approvalSample struct {name string; data []byte}
func makeApprovalSample(t *testing.T,items []approvalSample)string{
 t.Helper()
 path:=filepath.Join(t.TempDir(),"ALAA1156000000.zip")
 f,e:=os.Create(path);if e!=nil{t.Fatal(e)}
 zw:=zip.NewWriter(f)
 for _,item:=range items{
  h:=&zip.FileHeader{Name:item.name,Method:zip.Store}
  // Preserve raw CP950 bytes if the directory name is not valid UTF-8.
  if strings.Contains(item.name,zipSignCP950) || strings.Contains(item.name,zipDraftCP950) {h.NonUTF8=true}
  w,e:=zw.CreateHeader(h);if e!=nil{t.Fatal(e)}
  if _,e=w.Write(item.data);e!=nil{t.Fatal(e)}
 }
 if e=zw.Close();e!=nil{t.Fatal(e)}
 if e=f.Close();e!=nil{t.Fatal(e)}
 return path
}
func testApprovalPDF(t *testing.T)[]byte{
 t.Helper()
 b,e:=os.ReadFile("testdata/sample0.pdf");if e!=nil{t.Fatal(e)}
 return b
}

func TestZIPApprovalCP950SignAndDraftChoices(t *testing.T){
 pdf:=testApprovalPDF(t)
 path:=makeApprovalSample(t,[]approvalSample{
  {"來文\\letter.pdf",pdf},
  {zipSignCP950+"\\2-B.pdf",pdf},
  {zipSignCP950+"\\1156000000_2_00529553608(after).pdf",pdf},
  {zipSignCP950+"\\1-A.odt",[]byte("synthetic")},
  {zipSignCP950+"\\參考附件\\should-not-merge.pdf",pdf},
  {zipDraftCP950+"\\1156000000_3_123(after).pdf",pdf},
  {zipDraftCP950+"\\2-C.pdf",pdf},
 })
 modes,e:=availableZIPModes(path)
 if e!=nil{t.Fatal(e)}
 if len(modes)!=3||modes[0]!=zipModeLetter||modes[1]!=zipModeSign||modes[2]!=zipModeDraft{
  t.Fatalf("wrong modes: %v",modes)
 }
 sign,e:=analyzeZIPMode(path,zipModeSign)
 if e!=nil{t.Fatal(e)}
 if len(sign.Entries)!=3 {t.Fatalf("nested folder wrongly included: %+v",sign.Entries)}
 if !strings.Contains(sign.Entries[0].name,"(after).pdf")||sign.Entries[1].name!="1-A.odt"||sign.Entries[2].name!="2-B.pdf"{
  t.Fatalf("wrong sign sort: %+v",sign.Entries)
 }
 if name,e:=modePDFName(zipModeSign);e!=nil||name!="奉核簽.pdf"{t.Fatal(name,e)}
 draft,e:=analyzeZIPMode(path,zipModeDraft)
 if e!=nil{t.Fatal(e)}
 if len(draft.Entries)!=2||!strings.Contains(draft.Entries[0].name,"(after).pdf"){
  t.Fatalf("wrong draft sort: %+v",draft.Entries)
 }
 if name,e:=modePDFName(zipModeDraft);e!=nil||name!="奉核稿.pdf"{t.Fatal(name,e)}
}

func TestZIPApprovalExcludesNestedReferenceFiles(t *testing.T){
 pdf:=testApprovalPDF(t)
 path:=makeApprovalSample(t,[]approvalSample{
  {"簽\\1156000000_2_123(after).pdf",pdf},
  {"簽\\參考附件\\3.pdf",pdf},
  {"簽\\補充附件\\4.pdf",pdf},
  {"簽\\1.pdf",pdf},
 })
 job,e:=analyzeZIPMode(path,zipModeSign)
 if e!=nil{t.Fatal(e)}
 if len(job.Entries)!=2{t.Fatalf("nested files included: %+v",job.Entries)}
 out:=filepath.Join(t.TempDir(),"奉核簽.pdf")
 pages,e:=mergeOfficialZIP(job,out)
 if e!=nil||pages!=2{t.Fatalf("merge failed: %d %v",pages,e)}
 result,e:=parsePDF(out)
 if e!=nil||result.count!=2{t.Fatalf("unexpected pages: %d %v",pages,e)}
}

func TestZIPApprovalRejectsMissingAndDuplicateAfter(t *testing.T){
 pdf:=testApprovalPDF(t)
 for _,tc:=range []struct{name string;files []approvalSample}{
  {"missing",[]approvalSample{{"簽\\1.pdf",pdf}}},
  {"other_case",[]approvalSample{{"簽\\1156999999_1_123(after).pdf",pdf}}},
  {"duplicate",[]approvalSample{{"簽\\1156000000_1_123(after).pdf",pdf},{"簽\\1156000000_2_456(after).pdf",pdf}}},
  {"unsupported",[]approvalSample{{"簽\\1156000000_1_123(after).pdf",pdf},{"簽\\dangerous.exe",[]byte("not executed")}}},
 }{
  t.Run(tc.name,func(t *testing.T){
   path:=makeApprovalSample(t,tc.files)
   if _,e:=analyzeApprovalZIP(path,zipModeSign);e==nil{t.Fatal("expected safe failure")}
  })
 }
}

func TestZIPApprovalKeepsLegacyLetterParsing(t *testing.T){
 pdf:=testApprovalPDF(t)
 path:=makeApprovalSample(t,[]approvalSample{
  {"來文\\sample.pdf",pdf},
  {"簽\\1156000000_1_123(after).pdf",pdf},
 })
 letter,e:=analyzeZIPMode(path,zipModeLetter)
 if e!=nil{t.Fatal(e)}
 if len(letter.Entries)!=1||letter.Entries[0].name!="sample.pdf"{
  t.Fatalf("legacy 來文 flow changed: %+v",letter.Entries)
 }
}

func TestZIPApprovalCP950SecondByteBackslash(t *testing.T){
 // CP950 功 is A5 5C. The second byte must not split a path.
 pdf:=testApprovalPDF(t)
 specialName:=zipSignCP950+"\\1-\xa5\x5c.pdf"
 path:=makeApprovalSample(t,[]approvalSample{
  {zipSignCP950+"\\1156000000_1_123(after).pdf",pdf},
  {specialName,pdf},
 })
 job,e:=analyzeApprovalZIP(path,zipModeSign)
 if e!=nil{t.Fatal(e)}
 if len(job.Entries)!=2{t.Fatalf("CP950 attachment unexpectedly lost: %+v",job.Entries)}
 if root:=zipRootMode(specialName);root!=zipModeSign{t.Fatalf("wrong root for CP950 0x5c trail: %s",root)}
}
