package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIndirectReferences(t *testing.T) {
	in := []byte("<< /Contents 7 0 R /Parent 6 0 R /Resources << /Font 1 0 R /Note (7 0 R) /Hex <3720302052> >> >>")
	got := rewriteReferences(in, map[objKey]int{{7, 0}: 23, {6, 0}: 17, {1, 0}: 44})
	for _, item := range [][]byte{[]byte("/Contents 23 0 R"), []byte("/Parent 17 0 R"), []byte("/Font 44 0 R"), []byte("(7 0 R)"), []byte("<3720302052>")} {
		if !bytes.Contains(got, item) {
			t.Errorf("missing %s, output=%s", item, got)
		}
	}
}

func TestMergeSamplePDF(t *testing.T) {
	root := "testdata"
	files := []string{filepath.Join(root, "sample0.pdf"), filepath.Join(root, "sample1.pdf"), filepath.Join(root, "compressed.pdf")}
	for _, f := range files {
		if _, e := os.Stat(f); e != nil {
			t.Skip("test PDF files not supplied")
		}
	}
	out := filepath.Join(t.TempDir(), "merged.pdf")
	pages, err := mergePDFFiles(files, out)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 6 {
		t.Fatalf("want 6 pages, got %d", pages)
	}
	doc, err := parsePDF(out)
	if err != nil {
		t.Fatal(err)
	}
	if doc.count != 6 {
		t.Fatalf("new PDF says %d pages", doc.count)
	}
	out2 := filepath.Join(t.TempDir(), "merged_again.pdf")
	num, err := mergePDFFiles([]string{out, files[0]}, out2)
	if err != nil || num != 7 {
		t.Fatalf("remerge after previous merge: pages=%d err=%v", num, err)
	}
}
