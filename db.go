package main

import (
	"fmt"
	"os"

	"github.com/go-pg/pg/v10"
)

func StartDB() (*pg.DB, error) {
	var (
		opts *pg.Options
		err  error
	)

	//check if we are in prod
	//then use the db url from the env
	// if os.Getenv("ENV") == "PROD" {
	// 	opts, err = pg.ParseURL(os.Getenv("DATABASE_URL"))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// } else {
	// 	opts = &pg.Options{
	// 		//default port
	// 		//depends on the db service from docker compose
	// 		Addr:     "db:5432",
	// 		User:     "postgres",
	// 		Password: "admin",
	// 	}
	// }

	// Read database connection details from environment variables
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	conectionURL := fmt.Sprintf("%s:%s", dbHost, dbPort)
	fmt.Println("db connection url ", conectionURL)

	// Construct the connection options
	opts = &pg.Options{
		Addr:     conectionURL,
		User:     dbUser,
		Password: dbPassword,
		Database: dbName,
	} //"postgres://postgres:loop@007@34.122.49.254:5432/vault-db")

	//connect db
	db := pg.Connect(opts)

	return db, err
}
