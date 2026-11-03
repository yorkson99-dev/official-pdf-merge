package main

import (
 "archive/zip"
 "image"
 "image/color"
 "image/gif"
 "image/jpeg"
 "image/png"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func imageFixture(t *testing.T,dir,name string)string{
 t.Helper()
 img:=image.NewNRGBA(image.Rect(0,0,24,12))
 for y:=0;y<12;y++{for x:=0;x<24;x++{img.Set(x,y,color.NRGBA{R:byte(x*10),G:byte(y*18),B:20,A:255})}}
 path:=filepath.Join(dir,name)
 f,e:=os.Create(path);if e!=nil{t.Fatal(e)}
 switch strings.ToLower(filepath.Ext(name)){
 case ".jpg",".jpeg": e=jpeg.Encode(f,img,&jpeg.Options{Quality:90})
 case ".gif": e=gif.Encode(f,img,nil)
 default: e=png.Encode(f,img)
 }
 closeErr:=f.Close()
 if e!=nil{t.Fatal(e)}
 if closeErr!=nil{t.Fatal(closeErr)}
 return path
}

func TestImagePDFCommonStaticFormats(t *testing.T){
 for _,ext:=range []string{".png",".jpg",".jpeg",".gif"}{
  t.Run(ext,func(t *testing.T){
   dir:=t.TempDir()
   src:=imageFixture(t,dir,"photo"+ext)
   dst:=filepath.Join(dir,"out.pdf")
   if e:=convertDocumentToPDF(src,dst);e!=nil{t.Fatal(e)}
   p,e:=parsePDF(dst)
   if e!=nil||p.count!=1{t.Fatalf("image PDF must be one readable page: %v",e)}
   if e=convertDocumentToPDF(src,dst);e==nil{t.Fatal("must not overwrite existing PDF")}
  })
 }
}

func TestImageMergeWithPDFKeepsSequence(t *testing.T){
 dir:=t.TempDir()
 one:=imageFixture(t,dir,"a.png")
 two:=filepath.Join(dir,"b.pdf")
 b,e:=os.ReadFile("testdata/sample0.pdf");if e!=nil{t.Fatal(e)}
 if e=os.WriteFile(two,b,0600);e!=nil{t.Fatal(e)}
 three:=imageFixture(t,dir,"c.jpg")
 out:=filepath.Join(dir,"merged.pdf")
 n,e:=mergeMixedFiles([]string{one,two,three},out)
 if e!=nil||n!=3{t.Fatalf("expected three ordered pages, got %d (%v)",n,e)}
 pdf,e:=parsePDF(out)
 if e!=nil||pdf.count!=3{t.Fatalf("final merged PDF invalid: %v",e)}
}

func TestOfficeWarningExcludesImages(t *testing.T){
 names:=[]string{"a.png","b.JPG","c.gif","d.TIFF","e.webp","f.heic","g.avif","h.pdf","i.odt","j.docx"}
 if c:=countEditableDocuments(names);c!=2{t.Fatalf("expected only 2 Office conversions, got %d",c)}
 if needsMixedFileWarning(names){t.Fatal("images and PDF must not count as Office conversions")}
 names=append(names,"more.xlsx")
 if !needsMixedFileWarning(names){t.Fatal("3 Office files should warn")}
}

func TestAllImageExtensionsAreAccepted(t *testing.T){
 for ext:=range imageExts{
  if !supportedOfficialExt(ext){t.Fatalf("missing ZIP/manual support for %s",ext)}
  if !isImageExt(strings.ToUpper(ext)){t.Fatalf("case insensitive image recognition broken: %s",ext)}
 }
 if isImageExt(".svg"){t.Fatal("SVG must not be passed to raster decoder")}
}

func TestZIPImageAttachmentMergedWithoutOffice(t *testing.T){
 dir:=t.TempDir()
 src:=imageFixture(t,dir,"sample.png")
 imageBytes,e:=os.ReadFile(src);if e!=nil{t.Fatal(e)}
 pdfBytes,e:=os.ReadFile("testdata/sample0.pdf");if e!=nil{t.Fatal(e)}
 archive:=filepath.Join(dir,"letter.zip")
 f,e:=os.Create(archive);if e!=nil{t.Fatal(e)}
 zw:=zip.NewWriter(f)
 for _,item:=range []struct{name string;data []byte}{
  {"來文/case.pdf",pdfBytes},{"來文/case_ATTCH1.png",imageBytes},
 }{
  w,e:=zw.Create(item.name);if e!=nil{t.Fatal(e)}
  if _,e=w.Write(item.data);e!=nil{t.Fatal(e)}
 }
 if e=zw.Close();e!=nil{t.Fatal(e)}
 if e=f.Close();e!=nil{t.Fatal(e)}
 job,e:=analyzeZIP(archive);if e!=nil{t.Fatal(e)}
 if len(job.Entries)!=2||countEditableZIPDocuments(job.Entries)!=0{t.Fatal("image attachment incorrectly counted")}
 out:=filepath.Join(dir,"letter.pdf")
 count,e:=mergeOfficialZIP(job,out)
 if e!=nil||count!=2{t.Fatalf("image attachment ZIP merge failed: %d %v",count,e)}
}

func TestImageSizeValidation(t *testing.T){
 if checkImageDimensions(0,10)==nil||checkImageDimensions(40000001,1)==nil{t.Fatal("dimension cap must reject invalid images")}
}
