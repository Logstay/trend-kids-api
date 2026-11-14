// Comando para cadastrar o dono do painel. Não existe cadastro público por design.
//
//	ADMIN_PASSWORD='...' go run ./cmd/createadmin -email dono@exemplo.com
//
// Sem ADMIN_PASSWORD, pede a senha no terminal (sem eco). A senha nunca vai por argumento.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/Logstay/trend-kids-api/config"
	"github.com/Logstay/trend-kids-api/internal"
	ports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"golang.org/x/term"
)

func main() {
	email := flag.String("email", "", "e-mail do admin")
	flag.Parse()
	if *email == "" {
		log.Fatal("informe -email")
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		fmt.Fprint(os.Stderr, "Senha (mín. 12 caracteres): ")
		raw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			log.Fatalf("não consegui ler a senha: %v", err)
		}
		password = strings.TrimSpace(string(raw))
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("banco: %v", err)
	}
	defer db.Close()

	err = internal.NewCreateAdmin(db).Handle(context.Background(), ports.CreateAdminCommand{Email: *email, Password: password})
	if err != nil {
		log.Fatalf("erro: %v", err)
	}
	fmt.Println("admin criado")
}
