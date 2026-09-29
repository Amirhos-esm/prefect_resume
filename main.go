package main

import (
	"log"
	"os"

	"resume/internal/application"
)

func main() {
	if err := application.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
