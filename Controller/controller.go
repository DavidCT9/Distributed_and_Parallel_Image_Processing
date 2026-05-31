package controller

import (
	"fmt"
	"sync"
)

// this is how would be the master node
type Controller struct {
	DataStore    *DataStore
	Port         int
	nextWorkerID int
	mutex        sync.Mutex
}

// each time the code start it creates an empty new controller
func NewController(port int) *Controller {
	return &Controller{
		DataStore:    NewDataStore(),
		Port:         port,
		nextWorkerID: 0,
	}
}

// laucnhes the RPC server
func (c *Controller) StartController() {
	fmt.Printf("Controller RPC server starting on port %d : \n ", c.Port)
	go c.startRPC()

	select {}
}
