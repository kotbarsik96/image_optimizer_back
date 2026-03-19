package main

import (
	"image_optimizer/imgopt_s3"
	"log"

	"github.com/joho/godotenv"
)

var s3Actions imgopt_s3.S3Actions

func main() {
	godotenv.Load()

	s3Actions = imgopt_s3.S3Actions{}
	err := s3Actions.Init()
	if err != nil {
		log.Fatal(err)
	}

	DatabaseUp()

	RouterUp()
}
