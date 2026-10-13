package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestZIPClassification(t *testing.T) {
	if !fromLetterFolder("來文/one.pdf") || !fromLetterFolder("\xa8\xd3\xa4\xe5\\one.pdf") || fromLetterFolder("稿/one.pdf") {
		t.Fatal("folder detection")
	}
	for _, s := range []string{"name(opinion).pdf", "合併版.pdf", "name(after).pdf"} {
		if !excludedLetterFile(s) {
			t.Fatalf("expected excluded: %s", s)
		}
	}
}

// These tests are synthetic: no actual incoming official correspondence is committed.
func TestOrderedZIP(t *testing.T) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	source, e := os.ReadFile("testdata/sample0.pdf")
	if e != nil {
		t.Fatal(e)
	}
	cases := []string{
		"來文/0001_222_ATTCH10.pdf", "來文/0001_222.pdf",
		"來文/0001_222_ATTCH2.pdf", "來文/internal(opinion).pdf",
		"來文/合併版.pdf", "稿/another(after).pdf",
	}
	for _, n := range cases {
		f, e := w.Create(n)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Write(source); e != nil {
			t.Fatal(e)
		}
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	z := filepath.Join(t.TempDir(), "a.zip")
	if e := os.WriteFile(z, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	job, e := analyzeZIP(z)
	if e != nil {
		t.Fatal(e)
	}
	if len(job.Entries) != 3 {
		t.Fatalf("expected 3 PDFs, got %v", job.Entries)
	}
	for i, n := range []string{"0001_222.pdf", "0001_222_ATTCH2.pdf", "0001_222_ATTCH10.pdf"} {
		if job.Entries[i].name != n {
			t.Fatalf("entry %d = %s, want %s", i, job.Entries[i].name, n)
		}
	}
	out := filepath.Join(t.TempDir(), "merged.pdf")
	cnt, e := mergeOfficialZIP(job, out)
	if e != nil {
		t.Fatal(e)
	}
	if cnt != 3 {
		t.Errorf("merged pages %d", cnt)
	}
}

func TestNoAttachmentsZIP(t *testing.T) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, e := w.Create("來文/A_22.pdf")
	if e != nil {
		t.Fatal(e)
	}
	f.Write([]byte("%PDF-1.0\n"))
	w.Close()
	dir := t.TempDir()
	z := filepath.Join(dir, "z.zip")
	os.WriteFile(z, b.Bytes(), 0600)
	job, e := analyzeZIP(z)
	if e != nil {
		t.Fatal(e)
	}
	if len(job.Entries) != 1 {
		t.Fatal("main only failed")
	}
}
