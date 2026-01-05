package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"postgen/schema"
	"strings"
)

func main() {
	js := os.DirFS("jsonschema")
	updates, err := schema.InlineBundledSchemasInFS(js)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	for pa, out := range updates {
		_ = os.Remove(filepath.Join("jsonschema", pa))
		pa = strings.ReplaceAll(pa, ".jsonschema.strict.bundle", "")
		err = os.WriteFile(filepath.Join("jsonschema", pa), out, 0o644)
		if err != nil {
			slog.Error("Failed to write file", "err", err.Error(), "path", pa)
		}
	}
}
