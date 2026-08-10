// Exemplo completo: variáveis injetáveis via -ldflags + build constraints.
//
// Compilar com metadados:
//
//	go build -ldflags "-X main.Version=1.2.3 -X main.Commit=abc123" -o bin/exemplo .
//
// Cross-compile:
//
//	GOOS=linux GOARCH=amd64 go build -o bin/exemplo-linux-amd64 .
package main

import "fmt"

// Version e Commit são var (não const) para permitir sobrescrita via
// "go build -ldflags -X main.Version=... -X main.Commit=...".
var (
	Version = "dev"
	Commit  = "none"
)

func main() {
	fmt.Printf("version=%s commit=%s platform=%s\n", Version, Commit, Platform())
}
