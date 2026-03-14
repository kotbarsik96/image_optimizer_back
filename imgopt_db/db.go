package imgopt_db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type DatabaseWrapper struct {
	DB *sql.DB
}

func NewDatabaseWrapper() DatabaseWrapper {
	var dbwrapper DatabaseWrapper = DatabaseWrapper{}
	db, err := sql.Open("sqlite3", "./database.db")
	if err != nil {
		log.Fatalf("Could not open the database: %v", err)
	}
	dbwrapper.DB = db
	return dbwrapper
}

func (dw *DatabaseWrapper) Up() error {
	for _, creationStatement := range migrations {
		err := dw.createTable(creationStatement)
		if err != nil {
			log.Fatalf("Could not migrate tables: %v", err)
		}
	}

	return nil
}

func (dw *DatabaseWrapper) createTable(creationStatement string) error {
	st, err := dw.DB.Prepare(creationStatement)
	if err == nil {
		st.Exec()
	}
	return err
}
