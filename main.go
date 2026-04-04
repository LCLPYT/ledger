package main

import (
	"ledger/db"
	"ledger/routes"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	database := db.InitDB(dsn)
	defer database.Close()

	enforcer := db.InitCasbin(dsn)
	r := gin.Default()

	routes.SetupRoutes(r, enforcer, database)

	err := r.Run()

	if err != nil {
		os.Exit(1)
	}
}
