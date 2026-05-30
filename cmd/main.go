package main

import (
	"flag"
	"fmt"

	api "github.com/DavidCT9/Image_Filtering_API/API"
	controller "github.com/DavidCT9/Image_Filtering_API/Controller"
)

func main() {
	port := flag.Int("port", 40901, "Controller port")
	flag.Parse()

	fmt.Println("Welcome to the Distributed and Parallel Image Processing System")

	//creates and starts the master node
	c := controller.NewController(*port)
	go c.StartController()

	a := api.NewAPI(c, 8080)
	a.Start()
}
