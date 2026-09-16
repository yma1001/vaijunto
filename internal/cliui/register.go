package cliui

import (
	"fmt"

	"github.com/yma1001/vaijunto/internal/client"
)

// RegisterInteractive pede usuário/senha/confirmação e chama REGISTER.
// O papel vem do executável (motorista vs passageiro); o servidor gera o userId.
func RegisterInteractive(c *client.Client, read func(string) string, role string) {
	u := read("usuário: ")
	p := read("senha: ")
	conf := read("confirme a senha: ")
	if err := ValidateRegisterInput(u, p, conf); err != nil {
		fmt.Println(err)
		return
	}
	out, err := c.Register(u, p, role)
	if err != nil {
		fmt.Println(FriendlyError(err))
		return
	}
	fmt.Printf("Conta criada.\n  ID: %s\n  Usuário: %s\nVocê já pode entrar com esse usuário.\n", out.UserID, out.Username)
}
