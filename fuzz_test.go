package uyaml

import (
	"reflect"
	"testing"
)

// FuzzIndex differentially fuzzes the kernel-backed Index against the
// independent scalar reference. Any divergence is a bug by definition —
// the SIMD path must be bit-identical to the scalar truth.
func FuzzIndex(f *testing.F) {
	f.Add([]byte(fixture))
	f.Add([]byte(""))
	f.Add([]byte("--- !u!1 &-2 stripped\r\n\n"))
	f.Add([]byte("--- !u!929 &"))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := Index(data)
		if err != nil {
			t.Skip() // only ErrTooLarge, unreachable under fuzz sizes
		}
		if want := indexReference(data); !reflect.DeepEqual(got, want) {
			t.Fatalf("Index diverged from scalar reference on %q", data)
		}
	})
}
