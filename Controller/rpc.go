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
	Tags    []string
}

// the info that the controller (master node) sends back to the worker
type RegisterReply struct {
	WorkerID   int
	APIAddress string
	Token      string
}

type WorkerStatsArgs struct {
	WorkerID    int
	CPU         float64
	RAM         float64
	RunningJobs int
}

type WorkerStatsReply struct {
	OK bool
}

type ProcessImageJobArgs struct {
	WorkloadID string
	ImageID    string
	Filter     string
	APIAddress string
	Token      string
}

type ProcessImageJobReply struct {
	Success         bool
	FilteredImageID string
	Error           string
}

// RegisterWorker assigns an ID and records a worker in the datastore.
func (r *RPCController) RegisterWorker(args RegisterArgs, reply *RegisterReply) error {
	r.controller.mutex.Lock()
	id := r.controller.nextWorkerID
	r.controller.nextWorkerID++
	r.controller.mutex.Unlock()

	worker := Worker{
		ID:          id,
		Name:        args.Name,
		Address:     args.Address,
		Tags:        args.Tags,
		RunningJobs: 0,
	}

	r.controller.DataStore.AddWorker(worker)

	reply.WorkerID = id
	reply.APIAddress = r.controller.APIAddress
	reply.Token = r.controller.WorkerToken

	fmt.Printf("[INFO] worker %s has been registered with worker id: %d\n", args.Name, id)

	go r.controller.SchedulePendingJobs()
	return nil
}

func (r *RPCController) UpdateWorkerStats(args WorkerStatsArgs, reply *WorkerStatsReply) error {
	if err := r.controller.DataStore.UpdateWorkerStats(args.WorkerID, args.CPU, args.RAM, args.RunningJobs); err != nil {
		reply.OK = false
		return err
	}

	reply.OK = true
	return nil
}

// startRPC starts the server and serves each worker connection in its own goroutine.
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
