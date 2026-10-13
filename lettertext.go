package main

// Conservative, offline header reader for searchable digital PDFs.
// If a PDF is image-only or uses an unsupported font map, we do NOT invent a name.
import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

var cmapRange = regexp.MustCompile(`(?i)<([a-f0-9]{2,8})>\s*<([a-f0-9]{2,8})>\s*<([a-f0-9]{2,8})>`)
var cmapChar = regexp.MustCompile(`(?i)<([a-f0-9]{2,8})>\s*<([a-f0-9]{2,8})>`)
var unicodeRef = regexp.MustCompile(`/ToUnicode\s+(\d+)\s+(\d+)\s+R`)
var btBlock = regexp.MustCompile(`(?s)\bBT\b(.*?)\bET\b`)

func decodedStream(obj *pdfObject) []byte {
	dict := objectDict(obj.data)
	s := bytes.Index(obj.data, []byte("stream"))
	e := bytes.LastIndex(obj.data, []byte("endstream"))
	if s < 0 || e <= s {
		return nil
	}
	s += 6
	if s < len(obj.data) && obj.data[s] == '\r' {
		s++
	}
	if s < len(obj.data) && obj.data[s] == '\n' {
		s++
	}
	raw := obj.data[s:e]
	if m := directLengthPattern.FindSubmatch(dict); m != nil && !refLengthPattern.Match(dict) {
		n := atoi(m[1])
		if n <= len(raw) && n >= 0 {
			raw = raw[:n]
		}
	}
	if bytes.Contains(dict, []byte("/FlateDecode")) {
		r, er := zlib.NewReader(bytes.NewReader(raw))
		if er != nil {
			return nil
		}
		defer r.Close()
		data, er := io.ReadAll(io.LimitReader(r, 32*1024*1024))
		if er != nil {
			return nil
		}
		return data
	}
	if bytes.Contains(dict, []byte("/Filter")) {
		return nil
	}
	return raw
}
func utf16Hex(s []byte) string {
	b, e := hex.DecodeString(string(s))
	if e != nil {
		return ""
	}
	if len(b)%2 != 0 {
		return string(b)
	}
	vals := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		vals = append(vals, uint16(b[i])<<8|uint16(b[i+1]))
	}
	return string(utf16.Decode(vals))
}
func cmapForObject(b []byte) map[uint16]string {
	c := map[uint16]string{}
	for _, m := range cmapRange.FindAllSubmatch(b, -1) {
		a, e1 := strconv.ParseUint(string(m[1]), 16, 16)
		z, e2 := strconv.ParseUint(string(m[2]), 16, 16)
		base, e3 := strconv.ParseUint(string(m[3]), 16, 32)
		if e1 != nil || e2 != nil || e3 != nil || z < a || z-a > 2048 {
			continue
		}
		for v := a; v <= z; v++ {
			step := v - a
			// Common 4-digit BMP maps; preserve UTF-16 output for target PDFs.
			c[uint16(v)] = string(rune(base + step))
		}
	}
	for _, m := range cmapChar.FindAllSubmatch(b, -1) {
		a, e := strconv.ParseUint(string(m[1]), 16, 16)
		if e != nil {
			continue
		}
		if _, ok := c[uint16(a)]; ok {
			continue
		}
		v := utf16Hex(m[2])
		if v != "" {
			c[uint16(a)] = v
		}
	}
	return c
}
func unescapePDFLiteral(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' || i == len(b)-1 {
			out = append(out, b[i])
			continue
		}
		i++
		switch b[i] {
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case '\n':
		case '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				i++
			}
		default:
			if b[i] >= '0' && b[i] <= '7' {
				v := int(b[i] - '0')
				for k := 0; k < 2 && i+1 < len(b) && b[i+1] >= '0' && b[i+1] <= '7'; k++ {
					i++
					v = v*8 + int(b[i]-'0')
				}
				out = append(out, byte(v))
			} else {
				out = append(out, b[i])
			}
		}
	}
	return out
}
func decodeCIDText(b []byte, cm map[uint16]string) string {
	var s strings.Builder
	for i := 0; i+1 < len(b); i += 2 {
		n := uint16(b[i])<<8 | uint16(b[i+1])
		if v, ok := cm[n]; ok {
			s.WriteString(v)
		}
	}
	return s.String()
}
func extractTextBlock(b []byte, cm map[uint16]string) string {
	var out strings.Builder
	for i := 0; i < len(b); i++ {
		if b[i] == '(' {
			start := i + 1
			depth := 1
			for j := start; j < len(b); j++ {
				if b[j] == '\\' {
					j++
					continue
				}
				if b[j] == '(' {
					depth++
				}
				if b[j] == ')' {
					depth--
					if depth == 0 {
						unesc := unescapePDFLiteral(b[start:j])
						out.WriteString(decodeCIDText(unesc, cm))
						i = j
						break
					}
				}
			}
		} else if b[i] == '<' && i+1 < len(b) && b[i+1] != '<' {
			end := bytes.IndexByte(b[i+1:], '>')
			if end >= 0 {
				v, err := hex.DecodeString(string(bytes.ReplaceAll(b[i+1:i+1+end], []byte(" "), nil)))
				if err == nil {
					out.WriteString(decodeCIDText(v, cm))
				}
				i = i + 1 + end
			}
		}
	}
	return out.String()
}
func extractLetterHeader(blob []byte) (agency, date, docno, kind string) {
	f, err := os.CreateTemp("", "letterhdr-*.pdf")
	if err != nil {
		return
	}
	p := f.Name()
	defer os.Remove(p)
	if _, err = f.Write(blob); err != nil {
		f.Close()
		return
	}
	if err = f.Close(); err != nil {
		return
	}
	parsed, err := parsePDF(p)
	if err != nil {
		return
	}
	var maps []map[uint16]string
	for _, o := range parsed.objects {
		d := objectDict(o.data)
		m := unicodeRef.FindSubmatch(d)
		if m == nil {
			continue
		}
		key := objKey{atoi(m[1]), atoi(m[2])}
		cmapObj := parsed.objects[key]
		if cmapObj == nil {
			continue
		}
		cm := cmapForObject(decodedStream(cmapObj))
		if len(cm) > 0 {
			maps = append(maps, cm)
		}
	}
	if len(maps) == 0 {
		return
	}
	var best string
	for _, cm := range maps {
		var text strings.Builder
		for _, key := range parsed.order {
			o := parsed.objects[key]
			if o == nil {
				continue
			}
			d := objectDict(o.data)
			if !bytes.Contains(d, []byte("/Subtype /Form")) && !bytes.Contains(d, []byte("/Subtype/Form")) {
				continue
			}
			stream := decodedStream(o)
			for _, bt := range btBlock.FindAllSubmatch(stream, -1) {
				text.WriteString(extractTextBlock(bt[1], cm))
				text.WriteByte('\n')
			}
		}
		t := text.String()
		if strings.Contains(t, "發文日期") && strings.Contains(t, "發文字號") {
			best = t
			break
		}
		if len(t) > len(best) {
			best = t
		}
	}
	// Remove layout-only line breaks after labels; keep agency as a full line.
	if m := agencyLine.FindStringSubmatch(best); m != nil {
		candidate := strings.TrimSpace(m[1])
		candidate = strings.TrimPrefix(candidate, "\ufeff")
		han := false
		for _, r := range candidate {
			if unicode.Is(unicode.Han, r) {
				han = true
				break
			}
		}
		if han && !strings.Contains(candidate, "受文者") && !strings.Contains(candidate, "發文字號") && !strings.Contains(candidate, "發文日期") {
			agency = candidate
			kind = m[2]
		}
	}
	if m := dateLine.FindStringSubmatch(best); m != nil {
		date = m[1] + "年" + m[2] + "月" + m[3] + "日"
	}
	if m := numberLine.FindStringSubmatch(best); m != nil {
		docno = strings.Join(strings.Fields(m[1]), "")
	}
	if kind == "" {
		kind = "函"
	}
	return
}
