package main

import (
	"image_optimizer/imgopt_db"

	"github.com/joho/godotenv"
)

var dbwrapper imgopt_db.DatabaseWrapper
var rwrapper RouterWrapper
var utils Utils

func main() {
	godotenv.Load()

	utils = Utils{}
	dbwrapper = imgopt_db.NewDatabaseWrapper()
	rwrapper = NewRouterWrapper()

	dbwrapper.Up()

	rwrapper.Up()
}
