// Command gen-version-json gera o manifesto version.json de uma release.
//
// Uso (no workflow de release):
//
//	go run ./cmd/gen-version-json \
//	  --tag v1.2.3-dev \
//	  --notes-file notes.md \
//	  --checksums dist/checksums.txt \
//	  --min-supported v0.0.0 \
//	  --output dist/version.json
package main

import (
	"flag"
	"fmt"
	"os"

	"camera/internal/release"
)

func main() {
	tag := flag.String("tag", "", "tag da release (ex: v1.2.3-dev)")
	notesFile := flag.String("notes-file", "", "arquivo markdown com as notas da release")
	checksumsFile := flag.String("checksums", "", "arquivo checksums.txt (formato sha256sum)")
	minSupported := flag.String("min-supported", "", "versão mínima suportada")
	output := flag.String("output", "", "arquivo de saída (default: stdout)")
	flag.Parse()

	if err := run(*tag, *notesFile, *checksumsFile, *minSupported, *output); err != nil {
		fmt.Fprintln(os.Stderr, "gen-version-json:", err)
		os.Exit(1)
	}
}

func run(tag, notesFile, checksumsFile, minSupported, output string) error {
	if tag == "" {
		return fmt.Errorf("--tag é obrigatório")
	}

	notes, err := os.ReadFile(notesFile)
	if err != nil {
		return fmt.Errorf("ler notas: %w", err)
	}
	checksums, err := os.ReadFile(checksumsFile)
	if err != nil {
		return fmt.Errorf("ler checksums: %w", err)
	}

	m, err := release.BuildManifest(tag, string(notes), minSupported, string(checksums))
	if err != nil {
		return err
	}
	data, err := m.JSON()
	if err != nil {
		return fmt.Errorf("serializar manifesto: %w", err)
	}
	data = append(data, '\n')

	if output == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(output, data, 0o644)
}
