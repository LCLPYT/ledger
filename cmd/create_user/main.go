package main

import (
	"bufio"
	"fmt"
	"ledger/db"
	"ledger/util"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"

	_ "github.com/lib/pq"
	"golang.org/x/term"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	database := db.InitDB(dsn)
	defer database.Close()

	enforcer := db.InitCasbin(dsn)

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

	var id int64
	err = database.QueryRow(
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		username, email, hash,
	).Scan(&id)

	if err != nil {
		log.Fatal(err)
	}

	if _, err := enforcer.AddGroupingPolicy(strconv.FormatInt(id, 10), "default"); err != nil {
		log.Fatalf("Failed to assign default role: %v", err)
	}

	fmt.Printf("User created successfully with ID: %d\n", id)
}
