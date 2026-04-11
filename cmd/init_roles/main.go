package main

import (
	"fmt"
	"ledger/db"
	"ledger/perms"
	"log"
	"os"
	"strings"
)

func permPolicy(role, perm string) []string {
	parts := strings.SplitN(perm, ".", 2)
	return []string{role, parts[0], parts[1]}
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	database := db.InitDB(dsn)
	defer database.Close()

	enforcer := db.InitCasbin(dsn)

	for _, role := range []string{"default", "admin"} {
		_, err := database.Exec(
			"INSERT INTO roles (name, protected) VALUES ($1, TRUE) ON CONFLICT DO NOTHING", role,
		)
		if err != nil {
			log.Fatalf("Failed to create %s role: %v", role, err)
		}
	}

	defaultPerms := []string{perms.UserRead, perms.UserCreateToken}
	var policies [][]string
	for _, p := range defaultPerms {
		policies = append(policies, permPolicy("default", p))
	}
	policies = append(policies, []string{"admin", "*", "*"})

	_, err := enforcer.AddPolicies(policies)
	if err != nil {
		log.Fatalf("Failed to add policies: %v", err)
	}

	if err := enforcer.SavePolicy(); err != nil {
		log.Fatalf("Failed to save policy: %v", err)
	}

	fmt.Printf("Default role initialized with permissions: %s, %s\n",
		perms.UserRead, perms.UserCreateToken)
	fmt.Println("Admin role initialized with wildcard policy (*.*)")
}
