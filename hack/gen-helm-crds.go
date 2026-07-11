//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	srcDir := "config/crd/bases"
	dstDir := "dist/chart/templates/crd"

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: %s not found\n", srcDir)
		os.Exit(1)
	}

	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: %s not found\n", dstDir)
		os.Exit(1)
	}

	files, err := filepath.Glob(filepath.Join(srcDir, "*.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing CRDs: %v\n", err)
		os.Exit(1)
	}

	for _, src := range files {
		if err := processCRD(src, dstDir); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", src, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Synced %d CRDs to Helm chart\n", len(files))
}

func processCRD(srcPath, dstDir string) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	base := filepath.Base(srcPath)
	base = strings.TrimSuffix(base, ".yaml")

	// Extract kind from filename (e.g., "renovate.thegeeklab.de_renovators" -> "renovators")
	parts := strings.Split(base, "_")
	if len(parts) != 2 {
		return fmt.Errorf("unexpected filename format: %s", base)
	}

	kind := parts[1]
	dstName := kind + ".renovate.thegeeklab.de.yaml"
	dstPath := filepath.Join(dstDir, dstName)

	// Build Helm-wrapped content
	var b strings.Builder

	// Add Helm wrapper at the top
	b.WriteString("{{- if .Values.crd.enabled }}\n")

	// Add the source content
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		// Skip the --- separator
		if line == "---" {
			continue
		}
		// Skip the controller-gen annotation
		if strings.Contains(line, "controller-gen.kubebuilder.io/version:") {
			continue
		}
		// Add helm.sh/resource-policy annotation after metadata.annotations:
		if strings.TrimSpace(line) == "annotations:" && i > 0 && strings.Contains(lines[i-1], "metadata:") {
			b.WriteString(line)
			b.WriteString("\n")
			b.WriteString("    {{- if .Values.crd.keep }}\n")
			b.WriteString("    \"helm.sh/resource-policy\": keep\n")
			b.WriteString("    {{- end }}\n")
			continue
		}
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}

	// Add closing wrapper
	if !strings.HasSuffix(b.String(), "\n") {
		b.WriteString("\n")
	}
	b.WriteString("{{- end }}\n")

	// Check if content changed
	existing, _ := os.ReadFile(dstPath)
	if string(existing) == b.String() {
		fmt.Printf("no changes to %s\n", dstPath)
		return nil
	}

	if err := os.WriteFile(dstPath, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	fmt.Printf("updated %s\n", dstPath)
	return nil
}
