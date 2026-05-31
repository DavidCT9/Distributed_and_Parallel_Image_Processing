package main

import (
	"flag"
	"fmt"

	worker "github.com/DavidCT9/Image_Filtering_API/Worker"
)

func main() {
	controllerAddr := flag.String("controller", "localhost:40901", "Controller address")
	workerName := flag.String("worker-name", "worker0", "Worker name")
	flag.Parse()

	fmt.Printf("Starting worker %s\n", *workerName)

	w := worker.NewWorker(*workerName, *controllerAddr)
	w.Start()
}
