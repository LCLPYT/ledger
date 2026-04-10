package main

import (
	"fmt"
	"ledger/db"
	"ledger/permissions"
	"log"
	"os"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	database := db.InitDB(dsn)
	defer database.Close()

	enforcer := db.InitCasbin(dsn)

	_, err := database.Exec(
		"INSERT INTO roles (name) VALUES ('default') ON CONFLICT DO NOTHING",
	)
	if err != nil {
		log.Fatalf("Failed to create default role: %v", err)
	}

	policies := [][]string{
		{"default", "user", "read"},
		{"default", "user", "create_token"},
	}
	_, err = enforcer.AddPolicies(policies)
	if err != nil {
		log.Fatalf("Failed to add policies: %v", err)
	}

	if err := enforcer.SavePolicy(); err != nil {
		log.Fatalf("Failed to save policy: %v", err)
	}

	fmt.Printf("Default role initialized with permissions: %s, %s\n",
		permissions.UserRead, permissions.UserCreateToken)
}
