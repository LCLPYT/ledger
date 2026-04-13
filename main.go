package main

import (
	"fmt"
	"ledger/cmd"
	"ledger/db"
	"ledger/routes"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "user":
		cmd.RunUser(os.Args[2:])
	case "roles":
		cmd.RunRoles(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  ledger serve        Start the web server")
	fmt.Fprintln(os.Stderr, "  ledger user create  Create a new user interactively")
	fmt.Fprintln(os.Stderr, "  ledger roles init   Initialize default and admin roles")
}

func runServe() {
	dsn := os.Getenv("DATABASE_URL")
	database := db.InitDB(dsn)
	defer database.Close()

	enforcer := db.InitCasbin(dsn)
	r := gin.Default()

	routes.SetupRoutes(r, enforcer, database)

	if err := r.Run(); err != nil {
		os.Exit(1)
	}
}
