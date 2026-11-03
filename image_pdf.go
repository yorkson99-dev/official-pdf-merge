package main

// Raster images are rendered to a standard PDF without Office or any external
// process. PNG/JPEG/GIF use Go decoders; Windows WIC handles other formats.
import (
 "bytes"
 "compress/zlib"
 "errors"
 "fmt"
 "image"
 _ "image/gif"
 _ "image/jpeg"
 _ "image/png"
 "io"
 "os"
 "path/filepath"
 "strings"
)

const maxImagePixels=40000000
const maxImageFrames=200

var imageExts=map[string]bool{
 ".png":true, ".jpg":true, ".jpeg":true, ".jpe":true, ".jfif":true,
 ".gif":true, ".bmp":true, ".dib":true, ".tif":true, ".tiff":true,
 ".webp":true, ".heic":true, ".heif":true, ".hif":true, ".avif":true,
 ".ico":true, ".jxr":true, ".wdp":true, ".hdp":true,
}
func isImageExt(ext string)bool{return imageExts[strings.ToLower(ext)]}

type rasterFrame struct{width,height int;rgb []byte}

func checkImageDimensions(w,h int)error{
 if w<=0||h<=0||w>maxImagePixels/h{return errors.New("圖片尺寸過大或無效（最多4000萬像素／頁）")}
 return nil
}

func goRasterFrame(img image.Image)(rasterFrame,error){
 b:=img.Bounds();w,h:=b.Dx(),b.Dy()
 if e:=checkImageDimensions(w,h);e!=nil{return rasterFrame{},e}
 rgb:=make([]byte,w*h*3)
 i:=0
 for y:=b.Min.Y;y<b.Max.Y;y++{
  for x:=b.Min.X;x<b.Max.X;x++{
   // Color.RGBA returns premultiplied components, so C + (1-A)
   // correctly composites semitransparent pixels on white.
   r,g,bb,a:=img.At(x,y).RGBA()
   rgb[i]=uint8((r+65535-a)>>8)
   rgb[i+1]=uint8((g+65535-a)>>8)
   rgb[i+2]=uint8((bb+65535-a)>>8)
   i+=3
  }
 }
 return rasterFrame{w,h,rgb},nil
}

func imageFrames(path string)([]rasterFrame,error){
 ext:=strings.ToLower(filepath.Ext(path))
 if !isImageExt(ext){return nil,fmt.Errorf("不支援的圖片格式：%s",ext)}
 if ext==".png"||ext==".jpg"||ext==".jpeg"||ext==".jpe"||ext==".jfif"||ext==".gif"{
  f,e:=os.Open(path);if e!=nil{return nil,e};defer f.Close()
  config,_,e:=image.DecodeConfig(f)
  if e==nil{
   if e=checkImageDimensions(config.Width,config.Height);e!=nil{return nil,e}
   if _,e=f.Seek(0,io.SeekStart);e!=nil{return nil,e}
   img,_,e:=image.Decode(f)
   if e==nil{
    frame,e:=goRasterFrame(img)
    if e!=nil{return nil,e}
    return []rasterFrame{frame},nil
   }
  }
  // Fall back to WIC for a supported variant of the container format.
 }
 return decodeWindowsImageFrames(path,ext==".tif"||ext==".tiff")
}

func convertImageToPDF(input,output string)error{
 frames,e:=imageFrames(input)
 if e!=nil{return fmt.Errorf("圖片解碼失敗（可檢查 Windows 是否具備對應解碼器）：%w",e)}
 if len(frames)==0{return errors.New("圖片沒有可轉換的頁面")}
 if len(frames)>maxImageFrames{return errors.New("多頁 TIFF 超過200頁限制")}
 dest,e:=filepath.Abs(output);if e!=nil{return e}
 src,e:=filepath.Abs(input);if e!=nil{return e}
 if strings.EqualFold(src,dest){return errors.New("不可覆寫圖片來源")}
 f,e:=os.OpenFile(output,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
 if e!=nil{return e}
 ok:=false
 defer func(){_ = f.Close();if !ok{_ = os.Remove(output)}}()
 if e=writeRasterPDF(f,frames);e!=nil{return e}
 if e=f.Sync();e!=nil{return e}
 if e=f.Close();e!=nil{return e}
 ok=true
 return nil
}

func writeRasterPDF(out io.Writer,frames []rasterFrame)error{
 if len(frames)==0||len(frames)>maxImageFrames{return errors.New("圖片頁數無效")}
 var pdf bytes.Buffer
 offsets:=make([]int,3+3*len(frames))
 pdf.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
 object:=func(id int,data []byte){
  offsets[id]=pdf.Len()
  fmt.Fprintf(&pdf,"%d 0 obj\n",id)
  pdf.Write(data)
  pdf.WriteString("\nendobj\n")
 }
 object(1,[]byte("<< /Type /Catalog /Pages 2 0 R >>"))
 var kids strings.Builder
 for i:=range frames{fmt.Fprintf(&kids,"%d 0 R ",3+i*3)}
 object(2,[]byte(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [ %s] >>",len(frames),kids.String())))
 for i,frame:=range frames{
  if e:=checkImageDimensions(frame.width,frame.height);e!=nil{return e}
  if len(frame.rgb)!=frame.width*frame.height*3{return errors.New("圖片像素資料長度不正確")}
  pid,cid,iid:=3+i*3,4+i*3,5+i*3
  pw,ph:=595.0,842.0
  if frame.width>frame.height{pw,ph=842,595}
  availableW,availableH:=pw-48,ph-48
  scale:=availableW/float64(frame.width)
  if s:=availableH/float64(frame.height);s<scale{scale=s}
  w,h:=float64(frame.width)*scale,float64(frame.height)*scale
  x,y:=(pw-w)/2,(ph-h)/2
  object(pid,[]byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>",pw,ph,iid,cid)))
  content:=[]byte(fmt.Sprintf("q %.4f 0 0 %.4f %.4f %.4f cm /Im0 Do Q\n",w,h,x,y))
  contentData:=append([]byte(fmt.Sprintf("<< /Length %d >>\nstream\n",len(content))),content...)
  contentData=append(contentData,[]byte("endstream")...)
  object(cid,contentData)
  var compressed bytes.Buffer
  zw:=zlib.NewWriter(&compressed)
  if _,e:=zw.Write(frame.rgb);e!=nil{_ = zw.Close();return e}
  if e:=zw.Close();e!=nil{return e}
  prefix:=fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n",frame.width,frame.height,compressed.Len())
  imageData:=append([]byte(prefix),compressed.Bytes()...)
  imageData=append(imageData,[]byte("\nendstream")...)
  object(iid,imageData)
 }
 xref:=pdf.Len()
 fmt.Fprintf(&pdf,"xref\n0 %d\n0000000000 65535 f \n",len(offsets))
 for id:=1;id<len(offsets);id++{fmt.Fprintf(&pdf,"%010d 00000 n \n",offsets[id])}
 fmt.Fprintf(&pdf,"trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",len(offsets),xref)
 _,e:=io.Copy(out,&pdf)
 return e
}
