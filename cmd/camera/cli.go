package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"camera/internal/config"
)

// command descreve um subcomando da CLI para a ajuda (estilo docker).
type command struct {
	name    string
	summary string
}

func commands() []command {
	return []command{
		{"init", "Gera o camera.yaml (wizard interativo)"},
		{"config", "Mostra a configuração atual"},
		{"version", "Mostra a versão"},
		{"help", "Ajuda de qualquer comando"},
	}
}

// printTopHelp imprime a ajuda geral no layout do `docker --help`.
func printTopHelp(w io.Writer) {
	fmt.Fprint(w, "Usage:  camera [OPTIONS] COMMAND\n\n")
	fmt.Fprint(w, "Sistema de monitoramento residencial via RTSP.\n\n")
	fmt.Fprint(w, "Commands:\n")
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-9s %s\n", c.name, c.summary)
	}
	fmt.Fprint(w, "\nGlobal Options:\n")
	fmt.Fprint(w, "      --config string   Caminho do arquivo de config (padrão \"camera.yaml\")\n")
	fmt.Fprint(w, "  -v, --version         Mostra a versão e sai\n")
	fmt.Fprint(w, "\nRun 'camera COMMAND --help' for more information on a command.\n")
}

func initUsage() string {
	return "Usage:  camera init [OPTIONS]\n\n" +
		"Gera um camera.yaml interativamente (wizard).\n\n" +
		"Options:\n" +
		"      --output string   Caminho do arquivo a gerar (padrão \"camera.yaml\")\n"
}

func configUsage() string {
	keys := strings.Join(config.Config{}.EntryKeys(), ", ")
	return "Usage:  camera config [OPTIONS]\n\n" +
		"Mostra a configuração atual (lê o camera.yaml).\n\n" +
		"Options:\n" +
		"      --config string   Caminho do arquivo de config (padrão \"camera.yaml\")\n" +
		"      --get string      Chave a ler, ou 'all' (padrão \"all\")\n\n" +
		"Keys:\n  " + keys + "\n"
}

func versionUsage() string {
	return "Usage:  camera version\n\nMostra a versão, commit e data de build.\n"
}

// printCmdHelp imprime a ajuda de um comando específico (help <cmd> / <cmd> --help).
func printCmdHelp(name string) {
	switch name {
	case "init":
		fmt.Print(initUsage())
	case "config":
		fmt.Print(configUsage())
	case "version":
		fmt.Print(versionUsage())
	case "help":
		fmt.Print("Usage:  camera help [COMMAND]\n\nMostra a ajuda geral ou de um comando.\n")
	default:
		unknownCommand(name)
	}
}

func unknownCommand(name string) {
	fmt.Fprintf(os.Stderr, "camera: '%s' não é um comando. Veja 'camera help'.\n", name)
	os.Exit(1)
}

// hasHelpFlag reporta se algum arg é --help/-h.
func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

func runHelp(args []string) {
	if len(args) == 0 {
		printTopHelp(os.Stdout)
		return
	}
	printCmdHelp(args[0])
}

func printVersion() {
	fmt.Printf("camera %s (commit %s, built %s)\n", version, commit, builtAt)
}

// runConfig implementa `camera config [--config <path>] [--get <chave|all>]`.
func runConfig(args []string) {
	if hasHelpFlag(args) {
		fmt.Print(configUsage())
		return
	}
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	fs.Usage = func() { fmt.Print(configUsage()) }
	path := fs.String("config", "camera.yaml", "path to config file")
	get := fs.String("get", "all", "key to read, or 'all'")
	_ = fs.Parse(args)

	cfg, err := config.Load(*path)
	if err != nil {
		log.Fatal(err)
	}

	if *get == "" || *get == "all" {
		abs, aerr := filepath.Abs(*path)
		if aerr != nil {
			abs = *path
		}
		fmt.Printf("Arquivo: %s\n", abs)
		for _, kv := range cfg.Entries() {
			fmt.Printf("%s: %s\n", kv[0], kv[1])
		}
		return
	}

	for _, kv := range cfg.Entries() {
		if kv[0] == *get {
			fmt.Println(kv[1])
			return
		}
	}
	fmt.Fprintf(os.Stderr, "camera config: chave desconhecida %q. Válidas: %s\n",
		*get, strings.Join(cfg.EntryKeys(), ", "))
	os.Exit(1)
}
