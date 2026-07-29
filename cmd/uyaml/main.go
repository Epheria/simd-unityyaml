// uyaml is the demo/bench CLI for the simd-unityyaml library.
package main

import (
	"fmt"
	"os"
	"time"

	uyaml "github.com/epheria/simd-unityyaml"
)

func usage() {
	fmt.Fprintf(os.Stderr, `usage:
  uyaml index <file>   structural index summary (docs, lines, malformed)
  uyaml bench <file>   throughput of the stage-1 indexer on this machine
`)
	os.Exit(2)
}

func main() {
	if len(os.Args) != 3 {
		usage()
	}
	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "uyaml:", err)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "index":
		cmdIndex(data)
	case "bench":
		cmdBench(data)
	default:
		usage()
	}
}

func cmdIndex(data []byte) {
	tape, err := uyaml.Index(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uyaml:", err)
		os.Exit(1)
	}
	malformed := 0
	for _, d := range tape.Docs {
		if d.Malformed {
			malformed++
		}
	}
	fmt.Printf("KERNEL %s\nBYTES  %d\nLINES  %d\nDOCS   %d", uyaml.KernelName(), tape.N, len(tape.LineStarts), len(tape.Docs))
	if malformed > 0 {
		fmt.Printf(" (malformed: %d)", malformed)
	}
	fmt.Println()
	const cap = 20
	for i, d := range tape.Docs {
		if i == cap {
			fmt.Printf("more: %d docs hidden (pipe through your pager or use the library)\n", len(tape.Docs)-cap)
			break
		}
		if d.Malformed {
			fmt.Printf("  line %-6d MALFORMED header (prefix matched, rest unparsed)\n", d.Line+1)
			continue
		}
		suffix := ""
		if d.Stripped {
			suffix = " stripped"
		}
		fmt.Printf("  line %-6d !u!%-5d &%d%s\n", d.Line+1, d.ClassID, d.FileID, suffix)
	}
}

func cmdBench(data []byte) {
	const rounds = 20
	var tape *uyaml.Tape
	var err error
	start := time.Now()
	for i := 0; i < rounds; i++ {
		tape, err = uyaml.Index(data)
		if err != nil {
			fmt.Fprintln(os.Stderr, "uyaml:", err)
			os.Exit(1)
		}
	}
	elapsed := time.Since(start) / rounds
	mbs := float64(len(data)) / elapsed.Seconds() / (1 << 20)
	fmt.Printf("kernel=%s bytes=%d lines=%d docs=%d avg=%s throughput=%.0f MB/s (n=%d)\n",
		uyaml.KernelName(), len(data), len(tape.LineStarts), len(tape.Docs), elapsed, mbs, rounds)
}
