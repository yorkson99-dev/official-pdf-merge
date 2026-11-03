//go:build windows

package main

// Windows Imaging Component (WIC): use native or approved installed image
// codecs; do not spawn image editors, install codecs, or access the network.
import (
 "errors"
 "fmt"
 "runtime"
 "syscall"
 "unsafe"
)

type wicGUID struct{ Data1 uint32; Data2,Data3 uint16; Data4 [8]byte }
var (
 wicFactoryClass=wicGUID{0xcacaf262,0x9370,0x4615,[8]byte{0xa1,0x3b,0x9f,0x55,0x39,0xda,0x4c,0x0a}}
 wicFactoryIID=wicGUID{0xec5ec8a9,0xc395,0x4314,[8]byte{0x9c,0x77,0x54,0xd7,0xa9,0x35,0xff,0x70}}
 wicBGRA=wicGUID{0x6fddc324,0x4e03,0x4bfe,[8]byte{0xb1,0x85,0x3d,0x77,0x76,0x8d,0xc9,0x0f}}
 wicOLE=syscall.NewLazyDLL("ole32.dll")
 wicCoInit=wicOLE.NewProc("CoInitializeEx")
 wicCoUninit=wicOLE.NewProc("CoUninitialize")
 wicCoCreate=wicOLE.NewProc("CoCreateInstance")
)

func wicCall(obj uintptr,slot int,args ...uintptr)error{
 if obj==0{return errors.New("WIC 物件無效")}
 vt:=*(*uintptr)(unsafe.Pointer(obj))
 method:=*(*uintptr)(unsafe.Pointer(vt+uintptr(slot)*unsafe.Sizeof(uintptr(0))))
 params:=append([]uintptr{obj},args...)
 hr,_,_:=syscall.SyscallN(method,params...)
 if uint32(hr)&0x80000000!=0{return fmt.Errorf("WIC HRESULT 0x%08X",uint32(hr))}
 return nil
}
func wicRelease(obj uintptr){if obj!=0{_ =wicCall(obj,2)}}

func decodeWICFrame(factory,decoder uintptr,index uint32)(rasterFrame,error){
 var frame uintptr
 if err:=wicCall(decoder,13,uintptr(index),uintptr(unsafe.Pointer(&frame)));err!=nil{return rasterFrame{},err}
 defer wicRelease(frame)
 var width,height uint32
 if err:=wicCall(frame,3,uintptr(unsafe.Pointer(&width)),uintptr(unsafe.Pointer(&height)));err!=nil{return rasterFrame{},err}
 if err:=checkImageDimensions(int(width),int(height));err!=nil{return rasterFrame{},err}
 var converter uintptr
 if err:=wicCall(factory,10,uintptr(unsafe.Pointer(&converter)));err!=nil{return rasterFrame{},err}
 defer wicRelease(converter)
 // IWICFormatConverter.Initialize(source,pixelFormat,dither,palette,alpha,translate).
 if err:=wicCall(converter,8,frame,uintptr(unsafe.Pointer(&wicBGRA)),0,0,0,0);err!=nil{return rasterFrame{},err}
 pixels:=int(width)*int(height)
 raw:=make([]byte,pixels*4)
 if err:=wicCall(converter,7,0,uintptr(width*4),uintptr(len(raw)),uintptr(unsafe.Pointer(&raw[0])));err!=nil{return rasterFrame{},err}
 rgb:=make([]byte,pixels*3)
 for i:=0;i<pixels;i++{
  src,dst:=4*i,3*i
  a:=int(raw[src+3]);inv:=255-a
  rgb[dst]=byte((int(raw[src+2])*a+255*inv+127)/255)
  rgb[dst+1]=byte((int(raw[src+1])*a+255*inv+127)/255)
  rgb[dst+2]=byte((int(raw[src])*a+255*inv+127)/255)
 }
 return rasterFrame{int(width),int(height),rgb},nil
}

func decodeWindowsImageFrames(path string,multi bool)([]rasterFrame,error){
 runtime.LockOSThread()
 defer runtime.UnlockOSThread()
 hr,_,_:=wicCoInit.Call(0,2) // apartment-threaded COM on this OS thread
 if uint32(hr)&0x80000000!=0&&uint32(hr)!=0x80010106{
  return nil,fmt.Errorf("無法初始化 WIC：0x%08X",uint32(hr))
 }
 if uint32(hr)&0x80000000==0{defer wicCoUninit.Call()}
 var factory uintptr
 hr,_,_=wicCoCreate.Call(uintptr(unsafe.Pointer(&wicFactoryClass)),0,1,uintptr(unsafe.Pointer(&wicFactoryIID)),uintptr(unsafe.Pointer(&factory)))
 if uint32(hr)&0x80000000!=0{return nil,fmt.Errorf("無法建立 WIC 圖片解碼器：0x%08X",uint32(hr))}
 defer wicRelease(factory)
 name,e:=syscall.UTF16PtrFromString(path)
 if e!=nil{return nil,e}
 var decoder uintptr
 if e=wicCall(factory,3,uintptr(unsafe.Pointer(name)),0,0x80000000,0,uintptr(unsafe.Pointer(&decoder)));e!=nil{
  return nil,fmt.Errorf("Windows 無法開啟此圖片（可能未安裝對應解碼器）：%w",e)
 }
 defer wicRelease(decoder)
 var frameCount uint32
 if e=wicCall(decoder,12,uintptr(unsafe.Pointer(&frameCount)));e!=nil{return nil,e}
 if frameCount==0{return nil,errors.New("圖片沒有影格")}
 // GIF and animated WebP use the first frame; multipage TIFF keeps all pages.
 if !multi{frameCount=1}
 if frameCount>maxImageFrames{return nil,errors.New("TIFF 頁數超過上限200頁")}
 frames:=make([]rasterFrame,0,frameCount)
 for i:=uint32(0);i<frameCount;i++{
  frame,err:=decodeWICFrame(factory,decoder,i)
  if err!=nil{return nil,fmt.Errorf("圖片第%d頁解碼失敗：%w",i+1,err)}
  frames=append(frames,frame)
 }
 return frames,nil
}
