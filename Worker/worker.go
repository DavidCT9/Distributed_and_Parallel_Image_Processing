package worker

import (
	"fmt"
	"log"
	"net/rpc"
)

// Worker is the node that processes images
type Worker struct {
	Name           string
	ControllerAddr string
	APIAddress     string
	Token          string
	ID             int
	RPCClient      *rpc.Client
}

type RegisterArgs struct {
	Name    string
	Address string
}

type RegisterReply struct {
	WorkerID   int
	APIAddress string
	Token      string
}

// NewWorker creates a new worker with its name and controller address
func NewWorker(name, controllerAddr string) *Worker {
	return &Worker{
		Name:           name,
		ControllerAddr: controllerAddr,
	}
}

// Start connects the worker to the controller and registers itself
func (w *Worker) Start() {

	// connect to the controller via RPC
	client, err := rpc.Dial("tcp", w.ControllerAddr)
	if err != nil {
		log.Fatalf("Error connecting to controller: %v", err)
	}
	w.RPCClient = client

	fmt.Printf("Connected to controller at %s\n", w.ControllerAddr)

	// register with the controller
	args := RegisterArgs{
		Name:    w.Name,
		Address: fmt.Sprintf("localhost:%d", 50050+w.ID),
	}
	reply := RegisterReply{}

	err = w.RPCClient.Call("RPCController.RegisterWorker", args, &reply)
	if err != nil {
		log.Fatalf("Error registering worker: %v", err)
	}

	// save the info received from the controller
	w.ID = reply.WorkerID
	w.APIAddress = reply.APIAddress
	w.Token = reply.Token

	fmt.Printf("Worker %s registered with ID: %d\n", w.Name, w.ID)
	fmt.Printf("API address: %s\n", w.APIAddress)

	select {}
}
