package controller

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
)

type RPCController struct {
	controller *Controller
}

// the data that the workers show when they register
type RegisterArgs struct {
	Name    string
	Address string
}

// the info that the controller (master node) sends back to the worker
type RegisterReply struct {
	WorkerID   int
	APIAddress string
	Token      string
}

// reguster the worker asigns its ID
func (r *RPCController) RegisterWorker(args RegisterArgs, reply *RegisterReply) error {
	r.controller.mutex.Lock()
	id := r.controller.nextWorkerID
	r.controller.nextWorkerID++
	r.controller.mutex.Unlock()

	worker := Worker{
		ID:          id,
		Name:        args.Name,
		Adress:      args.Address,
		RunningJobs: 0,
	}

	r.controller.DataStore.AddWorker(worker)

	reply.WorkerID = id
	reply.APIAddress = "http://localhost:8080"
	reply.Token = "token"

	fmt.Printf("[INFO] worker %s has been registered with workers id: %d\n", args.Name, id)
	return nil
}

// starts thw server and listens to each worker in their own goroutine
func (c *Controller) startRPC() {
	rpc.Register(&RPCController{controller: c})

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		log.Fatal("Error starting RPC server:", err)
	}

	fmt.Printf("RPC server listening on port %d\n", c.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Connection error:", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}
