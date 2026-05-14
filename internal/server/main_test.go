package server_test

import (
	"os"
	"testing"

	"camera/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	db.BcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}
