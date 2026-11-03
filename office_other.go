//go:build !windows

package main

import ("fmt";"path/filepath")

func convertDocumentToPDF(input,output string)error{
 if isImageExt(filepath.Ext(input)){return convertImageToPDF(input,output)}
 return fmt.Errorf("僅支援在已安裝 Microsoft Office 的 Windows 電腦轉檔：%s",input)
}
