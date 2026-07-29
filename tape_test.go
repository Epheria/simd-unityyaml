package uyaml

import (
	"bytes"
	"reflect"
	"testing"
)

const fixture = `%YAML 1.1
%TAG !u! tag:unity3d.com,2011:
--- !u!1 &1234567890123456789
GameObject:
  m_Name: HomeWidget
  m_Component:
  - component: {fileID: 4}
--- !u!4 &-987654321 stripped
Transform:
  m_Father: {fileID: 0}
--- !u!114 &42
MonoBehaviour:
  m_Script: {fileID: 11500000, guid: abc, type: 3}
`

// indexReference is an independent scalar implementation of Index used as
// the differential oracle: no masks, no kernels, just a byte loop.
func indexReference(data []byte) *Tape {
	t := &Tape{N: len(data)}
	if len(data) == 0 {
		return t
	}
	t.LineStarts = append(t.LineStarts, 0)
	for i, b := range data {
		if b == '\n' && i+1 < len(data) {
			t.LineStarts = append(t.LineStarts, uint32(i+1))
		}
	}
	for i, start := range t.LineStarts {
		line := data[start:t.LineEnd(i)]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if d, ok := parseDocHeader(line, uint32(i)); ok {
			t.Docs = append(t.Docs, d)
		}
	}
	return t
}

func TestIndexFixture(t *testing.T) {
	tape, err := Index([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	want := []Doc{
		{ClassID: 1, FileID: 1234567890123456789, Line: 2},
		{ClassID: 4, FileID: -987654321, Line: 7, Stripped: true},
		{ClassID: 114, FileID: 42, Line: 10},
	}
	if !reflect.DeepEqual(tape.Docs, want) {
		t.Fatalf("docs mismatch\ngot  %+v\nwant %+v", tape.Docs, want)
	}
	if got := len(tape.LineStarts); got != 13 {
		t.Fatalf("line count = %d, want 13", got)
	}
}

func TestIndexMalformed(t *testing.T) {
	cases := []string{
		"--- !u!\n",           // no classID
		"--- !u!12\n",         // no space/anchor
		"--- !u!12 42\n",      // missing '&'
		"--- !u!12 &\n",       // no fileID digits
		"--- !u!12 &4x\n",     // trailing junk
		"--- !u!12 &4 strip\n", // bad suffix
		"--- !u!99999999999999999999 &4\n", // classID overflow
	}
	for _, src := range cases {
		tape, err := Index([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if len(tape.Docs) != 1 || !tape.Docs[0].Malformed {
			t.Fatalf("%q: want exactly one malformed doc, got %+v", src, tape.Docs)
		}
	}
	// Not a header at all: reported as nothing, not guessed as a doc.
	tape, err := Index([]byte("---!u!1 &2\n-- !u!1 &2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tape.Docs) != 0 {
		t.Fatalf("non-header lines produced docs: %+v", tape.Docs)
	}
}

func TestIndexEdges(t *testing.T) {
	for _, src := range []string{
		"",
		"\n",
		"--- !u!1 &2", // no trailing newline
		"--- !u!1 &2\r\n",
		"a\n\n\nb",
	} {
		got, err := Index([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		want := indexReference([]byte(src))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q:\ngot  %+v\nwant %+v", src, got, want)
		}
	}
}

func TestIndexMatchesReferenceLarge(t *testing.T) {
	data := bytes.Repeat([]byte(fixture), 500)
	got, err := Index(data)
	if err != nil {
		t.Fatal(err)
	}
	if want := indexReference(data); !reflect.DeepEqual(got, want) {
		t.Fatalf("large input diverged from scalar reference (docs %d vs %d, lines %d vs %d)",
			len(got.Docs), len(want.Docs), len(got.LineStarts), len(want.LineStarts))
	}
}
