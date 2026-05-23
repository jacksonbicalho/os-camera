package main

import (
	"errors"
	"fmt"
	"os"

	"camera/internal/updater"
)

func runUpdate(currentVersion string) {
	fmt.Printf("Versão atual: %s\n", currentVersion)
	fmt.Println("Verificando atualizações...")

	info, err := updater.CheckLatest(currentVersion, updater.DefaultAPIURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao verificar atualizações: %v\n", err)
		os.Exit(1)
	}

	if !info.UpdateAvailable {
		fmt.Println("Sistema já está na versão mais recente.")
		return
	}

	fmt.Printf("Nova versão disponível: %s\n", info.Latest)

	if updater.IsDocker() {
		fmt.Println("Rodando em container Docker.")
		fmt.Println("Para atualizar, execute no host:")
		fmt.Printf("  docker pull <imagem>:%s\n", info.Latest)
		fmt.Println("  docker-compose up -d  (ou equivalente)")
		return
	}

	fmt.Printf("Baixando e aplicando %s...\n", info.Latest)
	if err := updater.Apply(info); err != nil {
		if errors.Is(err, updater.ErrDocker) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Erro ao aplicar atualização: %v\n", err)
		os.Exit(1)
	}
	// se Apply retornar, o syscall.Exec falhou
	fmt.Fprintln(os.Stderr, "Atualização aplicada mas não foi possível reiniciar. Reinicie manualmente.")
	os.Exit(1)
}
