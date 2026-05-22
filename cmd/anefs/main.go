package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/stl3/anefs/internal/detect"
)

func main() {
	flag.Parse()

	path := flag.Arg(0)
	if path == "" {
		fmt.Println("usage: anefs <subtitle.ass>")
		os.Exit(1)
	}

	results, err := detect.FromFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println("no character names detected")
		return
	}

	fmt.Println("Detected character names:")
	fmt.Println()
	for _, r := range results {
		fmt.Printf("%-20s score=%d count=%d\n", r.Name, r.Score, r.Count)
	}
}
