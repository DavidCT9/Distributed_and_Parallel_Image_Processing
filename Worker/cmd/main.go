package main

import (
	"flag"
	"fmt"
	"strings"

	worker "github.com/DavidCT9/Image_Filtering_API/Worker"
)

func main() {
	controllerAddr := flag.String("controller", "localhost:40901", "Controller address")
	workerName := flag.String("worker-name", "worker0", "Worker name")
	tags := flag.String("tags", "cpu,grayscale,blur", "Comma-separated worker tags")
	flag.Parse()

	fmt.Printf("Starting worker %s\n", *workerName)

	w := worker.NewWorker(*workerName, *controllerAddr, parseTags(*tags))
	w.Start()
}

func parseTags(value string) []string {
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
