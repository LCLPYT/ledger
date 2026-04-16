package cmd

import (
	"bufio"
	"context"
	"fmt"
	appdb "ledger/db"
	dbsqlc "ledger/db/sqlc"
	"ledger/util"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

func RunUser(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  ledger user create  Create a new user interactively")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		runCreateUser()
	default:
		fmt.Fprintf(os.Stderr, "Unknown user command: %s\n", args[0])
		os.Exit(1)
	}
}

func runCreateUser() {
	dsn := os.Getenv("DATABASE_URL")
	pool := appdb.InitDB(dsn)
	defer pool.Close()
	queries := dbsqlc.New(pool)

	enforcer := appdb.InitCasbin(dsn)

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Enter Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println()

	hash, err := util.HashPassword(bytePassword)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("Grant admin privileges? [y/N]: ")
	adminInput, _ := reader.ReadString('\n')
	isAdmin := strings.TrimSpace(strings.ToLower(adminInput)) == "y"

	id, err := queries.AddUser(context.Background(), dbsqlc.AddUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
	})
	if err != nil {
		log.Fatal(err)
	}

	uid := strconv.FormatInt(id, 10)
	if _, err := enforcer.AddGroupingPolicy(uid, "default"); err != nil {
		log.Fatalf("Failed to assign default role: %v", err)
	}
	if isAdmin {
		if _, err := enforcer.AddGroupingPolicy(uid, "admin"); err != nil {
			log.Fatalf("Failed to assign admin role: %v", err)
		}
	}

	role := "default"
	if isAdmin {
		role = "default, admin"
	}
	fmt.Printf("User created successfully with ID: %d (roles: %s)\n", id, role)
}
