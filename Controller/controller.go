package controller

import (
	"fmt"
	"log"
	"net/rpc"
	"sync"

	scheduler "github.com/DavidCT9/Image_Filtering_API/Scheduler"
)

// Controller is the master node. It stores system state and schedules image
// processing jobs to connected workers.
type Controller struct {
	DataStore    *DataStore
	Port         int
	APIAddress   string
	WorkerToken  string
	nextWorkerID int
	mutex        sync.Mutex
	scheduleLock sync.Mutex
}

func NewController(port int) *Controller {
	return &Controller{
		DataStore:    NewDataStore(),
		Port:         port,
		APIAddress:   "http://localhost:8080",
		WorkerToken:  "worker-token",
		nextWorkerID: 0,
	}
}

func (c *Controller) SetAPIAddress(address string) {
	c.APIAddress = address
}

func (c *Controller) IsWorkerToken(token string) bool {
	return token != "" && token == c.WorkerToken
}

// StartController launches the controller RPC server.
func (c *Controller) StartController() {
	fmt.Printf("Controller RPC server starting on port %d : \n ", c.Port)
	go c.startRPC()

	select {}
}

// SchedulePendingJobs dispatches queued original images to the best available
// worker. It is serialized to avoid assigning the same image twice.
func (c *Controller) SchedulePendingJobs() {
	c.scheduleLock.Lock()
	defer c.scheduleLock.Unlock()

	for {
		queuedImages := c.DataStore.GetQueuedOriginalImages()
		if len(queuedImages) == 0 {
			return
		}

		worker, ok := c.selectWorker()
		if !ok {
			for _, image := range queuedImages {
				c.DataStore.RefreshWorkloadStatus(image.WorkloadID)
			}
			return
		}

		image, workload, started := c.DataStore.StartJob(queuedImages[0].ImageID, worker.ID)
		if !started {
			continue
		}

		go c.dispatchImageJob(worker, image, workload)
	}
}

func (c *Controller) selectWorker() (Worker, bool) {
	workers := c.DataStore.GetWorkers()
	if len(workers) == 0 {
		return Worker{}, false
	}

	candidates := make([]scheduler.WorkerCandidate, 0, len(workers))
	byID := make(map[int]Worker, len(workers))
	for _, worker := range workers {
		candidates = append(candidates, scheduler.WorkerCandidate{
			ID:          worker.ID,
			RunningJobs: worker.RunningJobs,
			CPU:         worker.CPU,
			RAM:         worker.RAM,
		})
		byID[worker.ID] = worker
	}

	selected, ok := scheduler.SelectWorker(candidates)
	if !ok {
		return Worker{}, false
	}

	return byID[selected.ID], true
}

func (c *Controller) dispatchImageJob(worker Worker, image Image, workload Workload) {
	client, err := rpc.Dial("tcp", worker.Address)
	if err != nil {
		log.Printf("error connecting to worker %s: %v", worker.Name, err)
		c.DataStore.FailJob(image.ImageID, worker.ID)
		return
	}
	defer client.Close()

	args := ProcessImageJobArgs{
		WorkloadID: image.WorkloadID,
		ImageID:    image.ImageID,
		Filter:     workload.Filter,
		APIAddress: c.APIAddress,
		Token:      c.WorkerToken,
	}
	reply := ProcessImageJobReply{}

	if err := client.Call("WorkerRPC.ProcessImageJob", args, &reply); err != nil {
		log.Printf("worker %s failed image %s: %v", worker.Name, image.ImageID, err)
		c.DataStore.FailJob(image.ImageID, worker.ID)
		return
	}
	if !reply.Success {
		log.Printf("worker %s failed image %s: %s", worker.Name, image.ImageID, reply.Error)
		c.DataStore.FailJob(image.ImageID, worker.ID)
		return
	}

	c.DataStore.CompleteJob(image.ImageID, worker.ID, reply.FilteredImageID)
}
