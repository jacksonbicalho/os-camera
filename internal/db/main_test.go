package db_test

import (
	"os"
	"testing"

	"camera/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	// bcrypt custo 12 é muito lento em testes; usar custo mínimo
	db.BcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}
