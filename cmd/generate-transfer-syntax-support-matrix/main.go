package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve working directory: %v\n", err)
		os.Exit(1)
	}

	type generatedDoc struct {
		path    string
		content string
	}

	docs := []generatedDoc{
		{
			path:    filepath.Join(root, "docs", "transfer-syntax-support-matrix.md"),
			content: transfersyntax.RenderSupportMatrixMarkdown(),
		},
		{
			path:    filepath.Join(root, "docs", "transfer-syntax-behavioral-summary.md"),
			content: transfersyntax.RenderBehavioralSummaryMarkdown(),
		},
	}

	for _, doc := range docs {
		if err := os.WriteFile(doc.path, []byte(doc.content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", doc.path, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", doc.path)
	}
}
