package main

import (
	"database/sql"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var gormDb *gorm.DB
var sqlDb *sql.DB

func DatabaseUp() {
	openedDb, err := gorm.Open(sqlite.Open("database.db?_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Could not open database: %v", err)
	}
	gormDb = openedDb

	openedSqlDb, err := gormDb.DB()
	if err != nil {
		log.Fatalf("Could not get sqlDb: %v", err)
	}
	sqlDb = openedSqlDb

	DatabaseMigrate()
}

func DatabaseMigrate() {
	migrator := gormDb.Migrator()

	migrator.AutoMigrate(&Uploader{}, &Project{}, &Folder{}, &Image{})
}
