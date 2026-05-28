package main

import (
	"flag"
	"fmt"

	controller "go.nanomsg.org/mangos/v3/protocol/pub/Controller"
)

func main() {
	port := flag.Int("port", 40901, "Controller port")
	flag.Parse()

	fmt.Println("Welcome to the Distributed and Parallel Image Processing System")

	//creates and starts the master node
	c := controller.NewController(*port)
	c.StartController()
}
