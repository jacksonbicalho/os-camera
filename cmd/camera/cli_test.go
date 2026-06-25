package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintTopHelp(t *testing.T) {
	var b bytes.Buffer
	printTopHelp(&b)
	out := b.String()
	for _, want := range []string{
		"Usage:  camera [OPTIONS] COMMAND",
		"Commands:",
		"init", "config", "version", "help",
		"Global Options:",
		"Run 'camera COMMAND --help' for more information on a command.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ajuda geral não contém %q\n---\n%s", want, out)
		}
	}
}

func TestConfigUsageListsKeys(t *testing.T) {
	out := configUsage()
	for _, want := range []string{
		"Usage:  camera config [OPTIONS]",
		"--get string",
		"Keys:",
		"server.port", "admin.username", "server.jwt_secret",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ajuda do config não contém %q\n---\n%s", want, out)
		}
	}
	// não deve oferecer chave de senha
	if strings.Contains(out, "admin.password") {
		t.Error("ajuda do config não deve listar admin.password")
	}
}
