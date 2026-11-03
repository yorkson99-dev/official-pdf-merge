//go:build !windows
package main
import "errors"
func decodeWindowsImageFrames(path string,multi bool)([]rasterFrame,error){
 return nil,errors.New("此圖片格式需要 Windows WIC 解碼器")
}
