package main

// Official-letter ZIP filtering, file ordering and naming. Pure standard library.
// ZIP entries are only read; no file is extracted outside a temporary private folder.

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type officialEntry struct {
	index       int
	name, group string
	number      int
	attachment  bool
}
type officialJob struct {
	ZipPath                        string
	Entries                        []officialEntry
	Agency, Date, DocumentNo, Kind string
	TextRead                       bool
}

var attachmentName = regexp.MustCompile(`(?i)^(.*?)_(?:ATTCH|ATTACH)([0-9]+)\.[^.]+$`)
var agencyLine = regexp.MustCompile(`(?m)^([^\r\n：:]{2,70}?)[ \t　]*(開會通知單|書函|通知|函)[ \t　]*$`)
var dateLine = regexp.MustCompile(`發文日期[：:\s]*(?:中華民國)?\s*([0-9]{2,3})\s*年\s*([0-9]{1,2})\s*月\s*([0-9]{1,2})\s*日`)
var numberLine = regexp.MustCompile(`發文字號[：:\s]*([^\r\n]*?號)`)

// ZIP legacy cp950 file names are inspected without lossy conversion.
// The ASCII suffix is entirely sufficient to detect ATTCH1, ATTCH2 etc.
// Chinese folder prefixes are recognized by their exact UTF-8 or CP950 bytes.
func fromLetterFolder(name string) bool {
	name = strings.ReplaceAll(name, `\`, `/`)
	p := strings.SplitN(name, `/`, 2)
	if len(p) != 2 {
		return false
	}
	// CP950 (Big5) bytes of 來文 are A8 D3 A4 E5.
	return p[0] == "來文" || p[0] == "\xa8\xd3\xa4\xe5"
}
func basenameZip(name string) string {
	name = strings.ReplaceAll(name, `\`, `/`)
	k := strings.LastIndexByte(name, '/')
	return name[k+1:]
}
func excludedLetterFile(name string) bool {
	l := strings.ToLower(name)
	return strings.Contains(l, "opinion") || strings.Contains(l, "(after)") || strings.Contains(l, "(before)") || strings.Contains(name, "合併版") || strings.Contains(name, "\xa6\x58\xa8\xd6\xaa\xa9")
}
// Supported correspondence attachments. Unknown ATTCH extensions are rejected,
// never silently dropped from a supposedly complete merged document.
func supportedOfficialExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".pdf", ".doc", ".docx", ".docm", ".odt", ".rtf", ".txt",
		".xls", ".xlsx", ".xlsm", ".ods", ".csv", ".tsv",
		".ppt", ".pptx", ".pptm", ".odp":
		return true
	}
	return false
}

func analyzeZIP(path string) (*officialJob, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("ZIP 無法開啟：%w", err)
	}
	defer r.Close()
	if len(r.File) > 4000 {
		return nil, errors.New("ZIP 檔案項目過多")
	}
	mains := map[string][]officialEntry{}
	atts := map[string][]officialEntry{}
	for i, f := range r.File {
		if f.FileInfo().IsDir() || !fromLetterFolder(f.Name) {
			continue
		}
		base := basenameZip(f.Name)
		if excludedLetterFile(base) {
			continue
		}
		if f.UncompressedSize64 > 700*1024*1024 {
			return nil, errors.New("檔案過大，已中止處理")
		}
		ext := strings.ToLower(filepath.Ext(base))
		n := strings.TrimSuffix(base, filepath.Ext(base))
		if matches := attachmentName.FindStringSubmatch(base); matches != nil {
			if !supportedOfficialExt(ext) {
				return nil, fmt.Errorf("附件 %s 不支援轉換為 PDF，已停止合併以避免遺漏", base)
			}
			var no int
			if _, err := fmt.Sscanf(matches[2], "%d", &no); err != nil || no <= 0 {
				continue
			}
			key := strings.ToLower(matches[1])
			atts[key] = append(atts[key], officialEntry{i, base, key, no, true})
		} else if ext == ".pdf" {
			key := strings.ToLower(n)
			mains[key] = append(mains[key], officialEntry{i, base, key, 0, false})
		}
	}
	if len(mains) == 0 {
		return nil, errors.New("來文資料夾找不到不含 ATTCH 後綴的主文 PDF，請確認 ZIP 格式")
	}
	if len(mains) != 1 {
		return nil, fmt.Errorf("來文資料夾找到 %d 組主文 PDF，為避免合併錯誤，請先整理檔案", len(mains))
	}
	var key string
	var main officialEntry
	for k, a := range mains {
		if len(a) != 1 {
			return nil, errors.New("存在重複主文 PDF，無法判定")
		}
		key = k
		main = a[0]
	}
	entries := []officialEntry{main}
	entries = append(entries, atts[key]...)
	sort.SliceStable(entries[1:], func(i, j int) bool { return entries[1+i].number < entries[1+j].number })
	// Reject duplicate attachment indices to avoid silently changing order.
	for i := 2; i < len(entries); i++ {
		if entries[i-1].number == entries[i].number {
			return nil, errors.New("出現重複的 ATTCH 編號，請先確認附件")
		}
	}
	job := &officialJob{ZipPath: path, Entries: entries}
	reader, err := r.File[main.index].Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	b, err := io.ReadAll(io.LimitReader(reader, 700*1024*1024+1))
	if err != nil {
		return nil, err
	}
	agency, date, num, kind := extractLetterHeader(b)
	job.Agency = agency
	job.Date = date
	job.DocumentNo = num
	job.Kind = kind
	job.TextRead = agency != "" && date != "" && num != ""
	return job, nil
}
func cleanComponent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "　 .")
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune("\\\\/:*?\"<>|", r) || r < 32 {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}
func (j *officialJob) outputName(detailed bool) (string, error) {
	agency := cleanComponent(j.Agency)
	if agency == "" {
		return "", errors.New("無法辨識來文機關，請改用手動命名")
	}
	if detailed {
		if j.Date == "" || j.DocumentNo == "" {
			return "", errors.New("無法完整辨識發文日期及發文字號，請使用一般命名或手動修改")
		}
		name := agency+j.Date+j.DocumentNo+j.Kind
        if len(j.Entries) > 1 {
            name += "及其附件"
        }
        return cleanComponent(name) + ".pdf", nil
	}
	suffix := "公文"
	if len(j.Entries) > 1 {
		suffix = "公文及其附件"
	}
	return agency + suffix + ".pdf", nil
}
// Test seams: the Office converter and workspace creation can be replaced in
// synthetic tests. No workspace is created for ordinary, small PDF-only ZIPs.
var convertEditable = convertDocumentToPDF
var convertEditableBatch = convertDocumentsToPDFBatch
const officeBatchLimit = 3
var createZIPWorkspace = os.MkdirTemp

// Keep regular ZIPs in memory. Large PDFs deliberately fall back to a private
// temporary workspace, so the tool does not exhaust memory on older PCs.
const zipPDFMemoryBudget uint64 = 96 * 1024 * 1024

func readZIPDocument(item *zip.File) ([]byte, error) {
    reader, err := item.Open()
    if err != nil { return nil, err }
    defer reader.Close()
    data, err := io.ReadAll(io.LimitReader(reader, int64(item.UncompressedSize64)+1))
    if err != nil { return nil, err }
    if uint64(len(data)) != item.UncompressedSize64 {
        return nil, errors.New("ZIP 內檔案解壓後大小不符")
    }
    return data, nil
}

func extractZIPDocument(item *zip.File, dest string) error {
    reader, err := item.Open()
    if err != nil { return err }
    defer reader.Close()
    writer, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if err != nil { return err }
    n, copyErr := io.Copy(writer, io.LimitReader(reader, int64(item.UncompressedSize64)+1))
    closeErr := writer.Close()
    if copyErr == nil { copyErr = closeErr }
    if copyErr == nil && uint64(n) != item.UncompressedSize64 {
        copyErr = errors.New("ZIP 內檔案解壓後大小不符")
    }
    if copyErr != nil {
        os.Remove(dest)
    }
    return copyErr
}

func mergeOfficialZIP(j *officialJob, out string) (int, error) {
    zr, err := zip.OpenReader(j.ZipPath)
    if err != nil { return 0, err }
    defer zr.Close()

    // Allocate a workspace only if a file requires Office conversion or an
    // unusually large PDF exceeds the in-memory processing budget.
    workspace := ""
    defer func() {
        if workspace != "" { os.RemoveAll(workspace) }
    }()
    getWorkspace := func() (string, error) {
        if workspace == "" {
            workspace, err = createZIPWorkspace("", "official-pdf-zip-*")
            if err != nil { return "", err }
        }
        return workspace, nil
    }

    docs := make([]*pdfInput, len(j.Entries))
    var total, memoryPDF uint64
    singleFile := ""
    var singleBytes []byte
    officeInputs := make([]string, 0)
    officeOutputs := make([]string, 0)
    officeIndices := make([]int, 0)

    // PDFs remain in memory wherever possible. Editable documents require
    // their original input and converted PDF on disk for the Office API.
    for i, entry := range j.Entries {
        if entry.index < 0 || entry.index >= len(zr.File) {
            return 0, errors.New("ZIP 內容已變更，請重新選擇")
        }
        item := zr.File[entry.index]
        if item.UncompressedSize64 > 700*1024*1024 {
            return 0, errors.New("附件超過大小限制")
        }
        if total > 1200*1024*1024-item.UncompressedSize64 {
            return 0, errors.New("全部附件超過 1.2 GB 限制")
        }
        total += item.UncompressedSize64

        ext := strings.ToLower(filepath.Ext(entry.name))
        if ext == ".pdf" && item.UncompressedSize64 <= zipPDFMemoryBudget-memoryPDF {
            data, e := readZIPDocument(item)
            if e != nil { return 0, fmt.Errorf("%s：%w", entry.name, e) }
            doc, e := parsePDFBytes(data, entry.name)
            if e != nil { return 0, fmt.Errorf("%s：%w", entry.name, e) }
            docs[i] = doc
            memoryPDF += uint64(len(data))
            if len(j.Entries) == 1 { singleBytes = data }
            continue
        }

        folder, e := getWorkspace()
        if e != nil { return 0, e }
        path := filepath.Join(folder, fmt.Sprintf("%04d%s", i, ext))
        if e := extractZIPDocument(item, path); e != nil {
            return 0, fmt.Errorf("解壓 %s：%w", entry.name, e)
        }
        if ext == ".pdf" {
            doc, e := parsePDF(path)
            if e != nil { return 0, fmt.Errorf("%s：%w", entry.name, e) }
            docs[i] = doc
            if len(j.Entries)==1 { singleFile = path }
        } else {
            officeInputs = append(officeInputs,path)
            officeOutputs = append(officeOutputs,filepath.Join(folder,fmt.Sprintf("%04d-converted.pdf",i)))
            officeIndices = append(officeIndices,i)
        }
    }

    // Convert no more than three editable documents per stage; COM remains
    // single-threaded. Each stage is parsed into memory before the next, with
    // no intermediate merged PDF written to disk.
    for base:=0;base<len(officeInputs);base+=officeBatchLimit {
        end:=base+officeBatchLimit
        if end>len(officeInputs) {end=len(officeInputs)}
        if end-base==1 {
            if e:=convertEditable(officeInputs[base],officeOutputs[base]);e!=nil {
                return 0,fmt.Errorf("附件 %s 轉成 PDF 失敗：%w",j.Entries[officeIndices[base]].name,e)
            }
        } else {
            if e:=convertEditableBatch(officeInputs[base:end],officeOutputs[base:end]);e!=nil{
                return 0,fmt.Errorf("第%d階段的 Office 附件轉檔失敗：%w",base/officeBatchLimit+1,e)
            }
        }
        for k:=base;k<end;k++ {
            i:=officeIndices[k]
            if e:=verifyConvertedPDF(officeOutputs[k]);e!=nil{
                return 0,fmt.Errorf("附件 %s 轉換結果無效：%w",j.Entries[i].name,e)
            }
            doc,e:=parsePDF(officeOutputs[k])
            if e!=nil{return 0,fmt.Errorf("%s：%w",j.Entries[i].name,e)}
            docs[i]=doc
        }
    }
    if len(docs) == 0 { return 0, errors.New("公文 ZIP 不含可合併的 PDF") }
    if len(docs) > 1 {
        return mergePDFDocuments(docs, out)
    }

    // One-PDF ZIPs preserve the original document bytes and metadata. Do not
    // create an intermediate PDF or modify an existing output.
    f, e := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if e != nil { return 0, e }
    var writeErr error
    if singleBytes != nil {
        _, writeErr = f.Write(singleBytes)
    } else {
        source, openErr := os.Open(singleFile)
        if openErr == nil {
            _, writeErr = io.Copy(f, source)
            source.Close()
        } else { writeErr = openErr }
    }
    closeErr := f.Close()
    if writeErr == nil { writeErr = closeErr }
    if writeErr != nil {
        os.Remove(out)
        return 0, writeErr
    }
    return docs[0].count, nil
}
