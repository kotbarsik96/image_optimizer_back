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

	// d := path.Join("_optimizations", "test", "optimization_3")
	// err = os.MkdirAll(d, 0666)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// _, err = os.Create(path.Join(d, "test.txt"))
	// if err != nil {
	// 	log.Fatal(err)
	// }

	DatabaseUp()

	RouterUp()
}
