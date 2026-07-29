package uyaml

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

// synthetic builds ~size bytes of Unity-shaped YAML: realistic line lengths,
// document density, and field noise.
func synthetic(size int) []byte {
	var buf bytes.Buffer
	buf.WriteString("%YAML 1.1\n%TAG !u! tag:unity3d.com,2011:\n")
	id := int64(1000000000000000000)
	for buf.Len() < size {
		fmt.Fprintf(&buf, "--- !u!1 &%d\nGameObject:\n  m_Name: Widget_%d\n  m_Layer: 0\n  m_Component:\n", id, id%97)
		for j := 0; j < 4; j++ {
			fmt.Fprintf(&buf, "  - component: {fileID: %d}\n", id+int64(j))
		}
		fmt.Fprintf(&buf, "--- !u!4 &%d\nTransform:\n  m_LocalPosition: {x: 0, y: 0, z: 0}\n  m_Father: {fileID: %d}\n  m_Children: []\n", id+10, id-10)
		id += 100
	}
	return buf.Bytes()
}

func benchInput(b *testing.B) []byte {
	if path := os.Getenv("UYAML_BENCH_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatalf("UYAML_BENCH_FILE: %v", err)
		}
		return data
	}
	return synthetic(8 << 20)
}

func BenchmarkNewlineMasks(b *testing.B) {
	data := benchInput(b)
	masks := make([]uint64, (len(data)+63)/64)
	b.Run(KernelName(), func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for i := 0; i < b.N; i++ {
			newlineMasks(data, masks)
		}
	})
	b.Run("swar-generic", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for i := 0; i < b.N; i++ {
			newlineMasksGeneric(data, masks)
		}
	})
}

func BenchmarkIndex(b *testing.B) {
	data := benchInput(b)
	b.Run("kernel", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for i := 0; i < b.N; i++ {
			if _, err := Index(data); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("scalar-reference", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		for i := 0; i < b.N; i++ {
			indexReference(data)
		}
	})
}
