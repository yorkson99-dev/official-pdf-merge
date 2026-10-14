package main

// Lightweight, dependency-free PDF object merger.
// The original PDF streams are copied byte for byte; page trees are grafted under
// a newly created root. This keeps original page contents, fonts and page sizes.

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type objKey struct{ id, generation int }
type pdfObject struct {
	key  objKey
	data []byte
}
type pdfInput struct {
	source  string
	objects map[objKey]*pdfObject
	root    objKey
	count   int
	order   []objKey
}

var (
	objectStart         = regexp.MustCompile(`(?m)(?:^|[\r\n])([0-9]{1,10})[ \t]+([0-9]{1,5})[ \t]+obj\b`)
	objRefPattern       = regexp.MustCompile(`([0-9]+)[ \t\r\n]+([0-9]+)[ \t\r\n]+R\b`)
	pagesPattern        = regexp.MustCompile(`/Pages\s+([0-9]+)\s+([0-9]+)\s+R\b`)
	rootPattern         = regexp.MustCompile(`/Root\s+([0-9]+)\s+([0-9]+)\s+R\b`)
	countPattern        = regexp.MustCompile(`/Count\s+([0-9]+)\b`)
	firstPattern        = regexp.MustCompile(`/First\s+([0-9]+)\b`)
	numPattern          = regexp.MustCompile(`/N\s+([0-9]+)\b`)
	typeCatalogPattern  = regexp.MustCompile(`/Type\s*/Catalog\b`)
	typeObjStmPattern   = regexp.MustCompile(`/Type\s*/ObjStm\b`)
	typeXrefPattern     = regexp.MustCompile(`/Type\s*/XRef\b`)
	typePagesPattern    = regexp.MustCompile(`/Type\s*/Pages\b`)
	filterPattern       = regexp.MustCompile(`/Filter\s*(?:\[\s*)?/(FlateDecode|ASCII85Decode|ASCIIHexDecode)\b`)
	refLengthPattern    = regexp.MustCompile(`/Length\s+([0-9]+)\s+([0-9]+)\s+R\b`)
	directLengthPattern = regexp.MustCompile(`/Length\s+([0-9]+)\b`)
)

func readNumber(src []byte, pattern *regexp.Regexp) (int, error) {
	m := pattern.FindSubmatch(src)
	if m == nil {
		return 0, errors.New("缺少必要 PDF 欄位")
	}
	n, e := strconv.Atoi(string(m[1]))
	return n, e
}

func objectDict(body []byte) []byte {
	if p := bytes.Index(body, []byte("stream")); p >= 0 {
		return body[:p]
	}
	return body
}

func startsWithWord(b []byte, i int, word string) bool {
	if i+len(word) > len(b) || string(b[i:i+len(word)]) != word {
		return false
	}
	if i+len(word) < len(b) {
		c := b[i+len(word)]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// extractOne reads an indirect object. Stream boundaries are discovered from
// endstream/endobj and, when available, a direct stream length. The body keeps
// original stream bytes unchanged.
func extractOne(all []byte, start int) (body []byte, until int, err error) {
	offset := start
	for offset < len(all) {
		end := bytes.Index(all[offset:], []byte("endobj"))
		if end < 0 {
			return nil, 0, errors.New("找不到 endobj")
		}
		end += offset
		content := all[start:end]
		streamMark := bytes.Index(content, []byte("stream"))
		if streamMark < 0 {
			return bytes.TrimSpace(content), end + len("endobj"), nil
		}
		// A valid stream object's endobj follows endstream. Endobj-like bytes
		// inside streams must not terminate the object.
		endStream := bytes.LastIndex(content, []byte("endstream"))
		if endStream >= streamMark {
			return bytes.TrimSpace(content), end + len("endobj"), nil
		}
		offset = end + len("endobj")
	}
	return nil, 0, errors.New("PDF 物件未正常結束")
}

func parsePDF(file string) (*pdfInput, error) {
	all, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parsePDFBytes(all, file)
}

// parsePDFBytes parses a PDF directly from memory. ZIP entries no longer need
// to be extracted to disk just to be passed to the object-level PDF merger.
func parsePDFBytes(all []byte, source string) (*pdfInput, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(all), []byte("%PDF-")) {
		return nil, errors.New("不是有效的 PDF 檔案")
	}
	if regexp.MustCompile(`/Encrypt(?:[ \t\r\n]*<<|[ \t\r\n]+[0-9]+\s+[0-9]+\s+R\b)`).Match(all) {
		return nil, errors.New("此 PDF 可能有加密保護，請先解除密碼限制")
	}
	if len(all) > 700*1024*1024 {
		return nil, errors.New("單一 PDF 超過 700 MB，已超出輕量版限制")
	}
	in := &pdfInput{source: source, objects: make(map[objKey]*pdfObject)}
	offset := 0
	for offset < len(all) {
		loc := objectStart.FindSubmatchIndex(all[offset:])
		if loc == nil {
			break
		}
		keyStart := offset + loc[0]
		start := offset + loc[1]
		id, _ := strconv.Atoi(string(all[offset+loc[2] : offset+loc[3]]))
		gen, _ := strconv.Atoi(string(all[offset+loc[4] : offset+loc[5]]))
		if id <= 0 {
			offset = start
			continue
		}
		body, after, e := extractOne(all, start)
		if e != nil {
			offset = keyStart + 1
			continue
		}
		key := objKey{id, gen}
		if _, exists := in.objects[key]; !exists {
			in.order = append(in.order, key)
		}
		in.objects[key] = &pdfObject{key, append([]byte(nil), body...)}
		offset = after
	}
	if len(in.objects) == 0 {
		return nil, errors.New("未找到 PDF 物件")
	}
	// Compressed object streams contain otherwise inaccessible PDF objects.
	// Decode them into standalone objects so all references can be relocated.
	for _, obj := range in.objects {
		if !typeObjStmPattern.Match(objectDict(obj.data)) {
			continue
		}
		if e := decodeObjectStream(in, obj); e != nil {
			return nil, fmt.Errorf("解壓 PDF 物件失敗：%w", e)
		}
	}
	// Respect the catalog referenced by the latest trailer. A PDF can
	// contain multiple catalogs due to incremental saves or earlier merges.
	var catalog *pdfObject
	if trailer := bytes.LastIndex(all, []byte("trailer")); trailer >= 0 {
		fragment := all[trailer:]
		if limit := bytes.Index(fragment, []byte("startxref")); limit >= 0 {
			fragment = fragment[:limit]
		}
		if m := rootPattern.FindSubmatch(fragment); m != nil {
			catalog = in.objects[objKey{atoi(m[1]), atoi(m[2])}]
		}
	}
	if catalog == nil {
		for _, key := range in.order {
			ob := in.objects[key]
			if ob != nil && typeXrefPattern.Match(objectDict(ob.data)) {
				if m := rootPattern.FindSubmatch(objectDict(ob.data)); m != nil {
					catalog = in.objects[objKey{atoi(m[1]), atoi(m[2])}]
				}
			}
		}
	}
	if catalog == nil {
		for _, key := range in.order {
			obj := in.objects[key]
			if obj != nil && typeCatalogPattern.Match(objectDict(obj.data)) {
				catalog = obj
			}
		}
	}
	if catalog == nil { // A catalog may appear inside an object stream.
		for _, obj := range in.objects {
			if typeCatalogPattern.Match(objectDict(obj.data)) {
				catalog = obj
				break
			}
		}
	}
	if catalog == nil {
		return nil, errors.New("找不到 PDF 目錄（Catalog）")
	}
	m := pagesPattern.FindSubmatch(objectDict(catalog.data))
	if m == nil {
		return nil, errors.New("找不到 PDF 頁面樹")
	}
	in.root = objKey{atoi(m[1]), atoi(m[2])}
	root := in.objects[in.root]
	if root == nil {
		return nil, errors.New("PDF 頁面樹物件不存在")
	}
	if !typePagesPattern.Match(objectDict(root.data)) {
		return nil, errors.New("PDF 頁面樹格式不符")
	}
	count, err := readNumber(objectDict(root.data), countPattern)
	in.count = count
	if err != nil || in.count < 1 {
		return nil, errors.New("無法確認 PDF 頁數")
	}
	return in, nil
}
func atoi(b []byte) int { n, _ := strconv.Atoi(string(b)); return n }

func decodeObjectStream(in *pdfInput, obj *pdfObject) error {
	dict := objectDict(obj.data)
	n, err := readNumber(dict, numPattern)
	if err != nil {
		return err
	}
	first, err := readNumber(dict, firstPattern)
	if err != nil {
		return err
	}
	if n < 0 || n > 1000000 {
		return errors.New("物件串流數量異常")
	}
	st := bytes.Index(obj.data, []byte("stream"))
	en := bytes.LastIndex(obj.data, []byte("endstream"))
	if st < 0 || en <= st {
		return errors.New("物件串流缺失")
	}
	st += 6
	if st < len(obj.data) && obj.data[st] == '\r' {
		st++
	}
	if st < len(obj.data) && obj.data[st] == '\n' {
		st++
	}
	data := obj.data[st:en]
	if m := directLengthPattern.FindSubmatch(dict); m != nil && !refLengthPattern.Match(dict) {
		sz := atoi(m[1])
		if sz >= 0 && sz <= len(data) {
			data = data[:sz]
		}
	}
	if filterPattern.Match(dict) {
		// Most PDF object streams use one FlateDecode filter.
		if !bytes.Contains(dict, []byte("/FlateDecode")) {
			return errors.New("暫不支援的物件壓縮方式")
		}
		zr, e := zlib.NewReader(bytes.NewReader(data))
		if e != nil {
			zr2 := flate.NewReader(bytes.NewReader(data))
			defer zr2.Close()
			dec, e2 := io.ReadAll(io.LimitReader(zr2, 100*1024*1024))
			if e2 != nil {
				return e
			}
			data = dec
		} else {
			dec, e2 := io.ReadAll(io.LimitReader(zr, 100*1024*1024))
			zr.Close()
			if e2 != nil {
				return e2
			}
			data = dec
		}
	} else if bytes.Contains(dict, []byte("/Filter")) {
		return errors.New("不支援的 PDF 物件串流編碼")
	}
	if first < 0 || first > len(data) {
		return errors.New("錯誤的 First 位移")
	}
	fields := bytes.Fields(data[:first])
	if len(fields) < n*2 {
		return errors.New("物件串流標頭不完整")
	}
	positions := make([]int, n)
	keys := make([]objKey, n)
	for i := 0; i < n; i++ {
		keys[i] = objKey{atoi(fields[2*i]), 0}
		positions[i] = first + atoi(fields[2*i+1])
		if positions[i] < first || positions[i] > len(data) {
			return errors.New("物件串流位移不正確")
		}
	}
	for i := 0; i < n; i++ {
		end := len(data)
		if i+1 < n {
			end = positions[i+1]
		}
		if end < positions[i] {
			return errors.New("物件串流排序錯誤")
		}
		k := keys[i]
		if _, exists := in.objects[k]; exists {
			continue
		}
		in.objects[k] = &pdfObject{k, bytes.TrimSpace(append([]byte(nil), data[positions[i]:end]...))}
		in.order = append(in.order, k)
	}
	return nil
}

// rewriteReferences rewrites only syntactic indirect references, never strings,
// comments, hex literals or binary streams.
func rewriteReferences(b []byte, mapping map[objKey]int) []byte {
	out := make([]byte, 0, len(b)+64)
	for i := 0; i < len(b); {
		c := b[i]
		if c == '%' {
			j := i + 1
			for j < len(b) && b[j] != '\n' && b[j] != '\r' {
				j++
			}
			out = append(out, b[i:j]...)
			i = j
			continue
		}
		if c == '(' {
			depth := 1
			j := i + 1
			for j < len(b) && depth > 0 {
				if b[j] == '\\' {
					j += 2
					continue
				}
				if b[j] == '(' {
					depth++
				} else if b[j] == ')' {
					depth--
				}
				j++
			}
			if j > len(b) {
				j = len(b)
			}
			out = append(out, b[i:j]...)
			i = j
			continue
		}
		if c == '<' && i+1 < len(b) && b[i+1] == '<' {
			out = append(out, '<', '<')
			i += 2
			continue
		}
		if c == '<' {
			j := i + 1
			for j < len(b) && b[j] != '>' {
				j++
			}
			if j < len(b) {
				j++
			}
			out = append(out, b[i:j]...)
			i = j
			continue
		}
		if c == '/' {
			j := i + 1
			for j < len(b) && !isDelimiter(b[j]) {
				j++
			}
			out = append(out, b[i:j]...)
			i = j
			continue
		}
		if c >= '0' && c <= '9' && (i == 0 || isDelimiter(b[i-1])) {
			if loc := objRefPattern.FindIndex(b[i:]); loc != nil && loc[0] == 0 {
				end := i + loc[1]
				if end == len(b) || isDelimiter(b[end]) {
					parts := bytes.Fields(b[i:end])
					if len(parts) == 3 {
						id := atoi(parts[0])
						g := atoi(parts[1])
						if newid, ok := mapping[objKey{id, g}]; ok {
							out = append(out, []byte(strconv.Itoa(newid)+" 0 R")...)
							i = end
							continue
						}
					}
				}
			}
		}
		out = append(out, c)
		i++
	}
	return out
}
func isDelimiter(b byte) bool {
	switch b {
	case ' ', '\n', '\r', '\t', '\f', 0, '/', '<', '>', '[', ']', '(', ')', '%':
		return true
	}
	return false
}

func relocateObject(obj *pdfObject, mapping map[objKey]int) []byte {
	b := obj.data
	p := bytes.Index(b, []byte("stream"))
	if p < 0 {
		return rewriteReferences(b, mapping)
	}
	// Don't change original byte streams. The dictionary may contain refs.
	out := rewriteReferences(b[:p], mapping)
	out = append(out, b[p:]...)
	return out
}

func appendParent(body []byte, newRoot int) ([]byte, error) {
	if bytes.Contains(objectDict(body), []byte("/Parent")) {
		// Existing /Parent of the original document's root is unusual but valid.
		// Replace only the reference in the top level /Pages dictionary.
		re := regexp.MustCompile(`/Parent\s+[0-9]+\s+[0-9]+\s+R\b`)
		if re.Match(objectDict(body)) {
			return re.ReplaceAll(body, []byte(fmt.Sprintf("/Parent %d 0 R", newRoot))), nil
		}
	}
	p := bytes.LastIndex(objectDict(body), []byte(">>"))
	if p < 0 {
		return nil, errors.New("PDF 頁面樹缺少結束標記")
	}
	ins := []byte(fmt.Sprintf(" /Parent %d 0 R ", newRoot))
	out := make([]byte, 0, len(body)+len(ins))
	out = append(out, body[:p]...)
	out = append(out, ins...)
	out = append(out, body[p:]...)
	return out, nil
}

// mergePDFFiles merges at the PDF object level. Work is only done on invocation;
// there are no temporary helpers, network calls, installer, or runtime dependencies.
func mergePDFFiles(inputs []string, output string) (int, error) {
	if len(inputs) < 2 {
		return 0, errors.New("至少需要 2 份 PDF")
	}
	destAbs, e := filepath.Abs(output)
	if e != nil {
		return 0, e
	}
	docs := make([]*pdfInput, 0, len(inputs))
	pages := 0
	for _, f := range inputs {
		abs, e := filepath.Abs(f)
		if e != nil {
			return 0, e
		}
		if strings.EqualFold(abs, destAbs) {
			return 0, errors.New("輸出檔不可與來源 PDF 相同")
		}
		doc, e := parsePDF(f)
		if e != nil {
			return 0, fmt.Errorf("%s：%w", filepath.Base(f), e)
		}
		if pages+doc.count > 100000 {
			return 0, errors.New("總頁數超出 100,000 頁上限")
		}
		pages += doc.count
		docs = append(docs, doc)
	}
	return mergePDFDocuments(docs, output)
}

// mergePDFDocuments writes already-parsed PDFs. It avoids reading input PDFs
// from temporary extracted files; the final sibling .tmp is retained for
// atomic publish and to avoid exposing partial official correspondence.
func mergePDFDocuments(docs []*pdfInput, output string) (int, error) {
	if len(docs) < 2 {
		return 0, errors.New("至少需要 2 份 PDF")
	}
	pages := 0
	for _, doc := range docs {
		if doc == nil || doc.count < 1 {
			return 0, errors.New("無效的 PDF 輸入")
		}
		if pages+doc.count > 100000 {
			return 0, errors.New("總頁數超出 100,000 頁上限")
		}
		pages += doc.count
	}
	// 1 = new document catalog, 2 = new page tree.
	next := 3
	mappings := make([]map[objKey]int, len(docs))
	for ix, doc := range docs {
		m := make(map[objKey]int, len(doc.objects))
		var keys []objKey
		for key, obj := range doc.objects {
			dict := objectDict(obj.data)
			if typeObjStmPattern.Match(dict) || typeXrefPattern.Match(dict) {
				continue
			}
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].id != keys[j].id {
				return keys[i].id < keys[j].id
			}
			return keys[i].generation < keys[j].generation
		})
		for _, k := range keys {
			m[k] = next
			next++
		}
		if _, ok := m[doc.root]; !ok {
			return 0, errors.New("找不到頁面樹根節點")
		}
		mappings[ix] = m
	}
	// Write to a temporary sibling file, avoiding partial/corrupt final outputs.
	parent := filepath.Dir(output)
	temp, err := os.CreateTemp(parent, ".fastpdfmerge-*.tmp")
	if err != nil {
		return 0, err
	}
	tmpName := temp.Name()
	success := false
	defer func() {
		temp.Close()
		if !success {
			os.Remove(tmpName)
		}
	}()
	writer := &pdfWriter{out: temp, offsets: make([]int64, next)}
	if e := writer.write([]byte("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n")); e != nil {
		return 0, e
	}
	kids := make([]string, len(docs))
	for i, doc := range docs {
		kids[i] = fmt.Sprintf("%d 0 R", mappings[i][doc.root])
	}
	if e := writer.object(1, []byte("<< /Type /Catalog /Pages 2 0 R >>")); e != nil {
		return 0, e
	}
	if e := writer.object(2, []byte(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [ %s ] >>", pages, strings.Join(kids, " ")))); e != nil {
		return 0, e
	}
	for ix, doc := range docs {
		m := mappings[ix]
		keys := make([]objKey, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return m[keys[i]] < m[keys[j]] })
		for _, key := range keys {
			body := relocateObject(doc.objects[key], m)
			if key == doc.root {
				parented, err := appendParent(body, 2)
				if err != nil {
					return 0, err
				}
				body = parented
			}
			if e := writer.object(m[key], body); e != nil {
				return 0, e
			}
		}
	}
	if e := writer.finish(1); e != nil {
		return 0, e
	}
	if e := temp.Sync(); e != nil {
		return 0, e
	}
	if e := temp.Close(); e != nil {
		return 0, e
	}
	// Never overwrite an existing output PDF unexpectedly.
	if _, e := os.Stat(output); e == nil {
		return 0, errors.New("輸出檔已存在，請更換檔名")
	}
	if e := os.Rename(tmpName, output); e != nil {
		return 0, e
	}
	success = true
	return pages, nil
}

type pdfWriter struct {
	out     *os.File
	offset  int64
	offsets []int64
}

func (w *pdfWriter) write(b []byte) error { n, e := w.out.Write(b); w.offset += int64(n); return e }
func (w *pdfWriter) object(id int, b []byte) error {
	w.offsets[id] = w.offset
	if e := w.write([]byte(fmt.Sprintf("%d 0 obj\n", id))); e != nil {
		return e
	}
	if e := w.write(b); e != nil {
		return e
	}
	return w.write([]byte("\nendobj\n"))
}
func (w *pdfWriter) finish(root int) error {
	xref := w.offset
	if e := w.write([]byte(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(w.offsets)))); e != nil {
		return e
	}
	for id := 1; id < len(w.offsets); id++ {
		if w.offsets[id] > 9999999999 {
			return errors.New("PDF 超過 10 GB，超出規格")
		}
		if e := w.write([]byte(fmt.Sprintf("%010d 00000 n \n", w.offsets[id]))); e != nil {
			return e
		}
	}
	return w.write([]byte(fmt.Sprintf("trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(w.offsets), root, xref)))
}
