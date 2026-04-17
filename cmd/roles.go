package cmd

import (
	"context"
	"fmt"
	"ledger/db"
	dbsqlc "ledger/db/sqlc"
	"ledger/perms"
	"log"
	"os"
	"strings"
)

func RunRoles(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  ledger roles init  Initialize default and admin roles")
		os.Exit(1)
	}
	switch args[0] {
	case "init":
		runInitRoles()
	default:
		fmt.Fprintf(os.Stderr, "Unknown roles command: %s\n", args[0])
		os.Exit(1)
	}
}

func runInitRoles() {
	dsn := os.Getenv("DATABASE_URL")
	pool := db.InitDB(dsn)
	defer pool.Close()
	q := dbsqlc.New(pool)

	enforcer := db.InitCasbin(dsn)

	for _, role := range []string{"default", "admin"} {
		if err := q.InitRole(context.Background(), role); err != nil {
			log.Fatalf("Failed to create %s role: %v", role, err)
		}
	}

	permPolicy := func(role, perm string) []string {
		parts := strings.SplitN(perm, ".", 2)
		return []string{role, parts[0], parts[1]}
	}

	defaultPerms := []string{perms.UserRead, perms.UserCreateToken}
	var policies [][]string
	for _, p := range defaultPerms {
		policies = append(policies, permPolicy("default", p))
	}
	policies = append(policies, []string{"admin", "*", "*"})

	if _, err := enforcer.AddPolicies(policies); err != nil {
		log.Fatalf("Failed to add policies: %v", err)
	}

	if err := enforcer.SavePolicy(); err != nil {
		log.Fatalf("Failed to save policy: %v", err)
	}

	fmt.Printf("Default role initialized with permissions: %s, %s\n",
		perms.UserRead, perms.UserCreateToken)
	fmt.Println("Admin role initialized with wildcard policy (*.*)")
}
