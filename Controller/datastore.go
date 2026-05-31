package controller

import (
	"errors"
	"sync"
)

const (
	WorkloadScheduling = "scheduling"
	WorkloadRunning    = "running"
	WorkloadCompleted  = "completed"

	ImageOriginal = "original"
	ImageFiltered = "filtered"

	ImageQueued    = "queued"
	ImageRunning   = "running"
	ImageCompleted = "completed"
	ImageFailed    = "failed"
)

var ErrNotFound = errors.New("not found")

// Worker stores the state that the controller knows about each worker node.
type Worker struct {
	ID          int      `json:"worker_id"`
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	CPU         float64  `json:"cpu"`
	RAM         float64  `json:"ram"`
	RunningJobs int      `json:"running_jobs"`
	Tags        []string `json:"tags"`
}

// Workload stores the filtering request created by a client.
type Workload struct {
	ID             string   `json:"workload_id"`
	Name           string   `json:"workload_name"`
	Directory      string   `json:"-"`
	Filter         string   `json:"filter"`
	Status         string   `json:"status"`
	RunningJobs    int      `json:"running_jobs"`
	OriginalImages []string `json:"original_images,omitempty"`
	FilteredImages []string `json:"filtered_images"`
}

type Image struct {
	ImageID          string `json:"image_id"`
	WorkloadID       string `json:"workload_id"`
	Type             string `json:"type"`
	Path             string `json:"-"`
	Status           string `json:"status,omitempty"`
	OriginalImageID  string `json:"original_image_id,omitempty"`
	AssignedWorkerID int    `json:"-"`
}

// DataStore saves the global state of the system.
type DataStore struct {
	Workers   map[int]Worker
	Workloads map[string]Workload
	Images    map[string]Image
	mutex     sync.RWMutex
}

func NewDataStore() *DataStore {
	return &DataStore{
		Workers:   make(map[int]Worker),
		Workloads: make(map[string]Workload),
		Images:    make(map[string]Image),
	}
}

// AddWorker registers or replaces a worker in the datastore.
func (d *DataStore) AddWorker(w Worker) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Workers[w.ID] = w
}

func (d *DataStore) UpdateWorkerStats(id int, cpu, ram float64, runningJobs int) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	worker, ok := d.Workers[id]
	if !ok {
		return ErrNotFound
	}

	worker.CPU = cpu
	worker.RAM = ram
	if runningJobs >= 0 {
		worker.RunningJobs = runningJobs
	}
	d.Workers[id] = worker
	return nil
}

func (d *DataStore) AddWorkerRunningJob(id int, delta int) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	worker, ok := d.Workers[id]
	if !ok {
		return
	}
	worker.RunningJobs += delta
	if worker.RunningJobs < 0 {
		worker.RunningJobs = 0
	}
	d.Workers[id] = worker
}

func (d *DataStore) AddWorkload(w Workload) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Workloads[w.ID] = w
}

func (d *DataStore) AddImage(i Image) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if i.Status == "" {
		if i.Type == ImageOriginal {
			i.Status = ImageQueued
		} else {
			i.Status = ImageCompleted
		}
	}

	d.Images[i.ImageID] = i

	workload, ok := d.Workloads[i.WorkloadID]
	if !ok {
		return
	}

	if i.Type == ImageOriginal {
		workload.OriginalImages = appendIfMissing(workload.OriginalImages, i.ImageID)
		if workload.Status == "" || workload.Status == WorkloadCompleted {
			workload.Status = WorkloadScheduling
		}
	}

	if i.Type == ImageFiltered {
		workload.FilteredImages = appendIfMissing(workload.FilteredImages, i.ImageID)
	}

	d.Workloads[workload.ID] = workload
}

func (d *DataStore) GetWorkers() []Worker {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	workers := make([]Worker, 0, len(d.Workers))
	for _, worker := range d.Workers {
		workers = append(workers, worker)
	}
	return workers
}

func (d *DataStore) GetWorkload(id string) (Workload, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	workload, ok := d.Workloads[id]
	return workload, ok
}

func (d *DataStore) GetAllWorkloads() []Workload {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	workloads := make([]Workload, 0, len(d.Workloads))
	for _, workload := range d.Workloads {
		workloads = append(workloads, workload)
	}
	return workloads
}

func (d *DataStore) GetImage(id string) (Image, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	image, ok := d.Images[id]
	return image, ok
}

func (d *DataStore) GetAllImages() []Image {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	images := make([]Image, 0, len(d.Images))
	for _, image := range d.Images {
		images = append(images, image)
	}
	return images
}

func (d *DataStore) GetQueuedOriginalImages() []Image {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	images := []Image{}
	for _, image := range d.Images {
		if image.Type == ImageOriginal && image.Status == ImageQueued {
			images = append(images, image)
		}
	}
	return images
}

func (d *DataStore) StartJob(imageID string, workerID int) (Image, Workload, bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	image, imageOK := d.Images[imageID]
	workload, workloadOK := d.Workloads[image.WorkloadID]
	worker, workerOK := d.Workers[workerID]
	if !imageOK || !workloadOK || !workerOK || image.Status != ImageQueued {
		return Image{}, Workload{}, false
	}

	image.Status = ImageRunning
	image.AssignedWorkerID = workerID
	d.Images[imageID] = image

	worker.RunningJobs++
	d.Workers[workerID] = worker

	workload.RunningJobs++
	workload.Status = WorkloadRunning
	d.Workloads[workload.ID] = workload

	return image, workload, true
}

func (d *DataStore) CompleteJob(originalImageID string, workerID int, filteredImageID string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	image, ok := d.Images[originalImageID]
	if ok {
		image.Status = ImageCompleted
		d.Images[originalImageID] = image
	}

	d.decrementWorkerLocked(workerID)

	workload, ok := d.Workloads[image.WorkloadID]
	if !ok {
		return
	}

	if workload.RunningJobs > 0 {
		workload.RunningJobs--
	}
	if filteredImageID != "" {
		workload.FilteredImages = appendIfMissing(workload.FilteredImages, filteredImageID)
	}
	workload.Status = d.calculateWorkloadStatusLocked(workload)
	d.Workloads[workload.ID] = workload
}

func (d *DataStore) FailJob(originalImageID string, workerID int) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	image, ok := d.Images[originalImageID]
	if ok {
		image.Status = ImageFailed
		d.Images[originalImageID] = image
	}

	d.decrementWorkerLocked(workerID)

	workload, ok := d.Workloads[image.WorkloadID]
	if !ok {
		return
	}
	if workload.RunningJobs > 0 {
		workload.RunningJobs--
	}
	workload.Status = d.calculateWorkloadStatusLocked(workload)
	d.Workloads[workload.ID] = workload
}

func (d *DataStore) RefreshWorkloadStatus(workloadID string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	workload, ok := d.Workloads[workloadID]
	if !ok {
		return
	}
	workload.Status = d.calculateWorkloadStatusLocked(workload)
	d.Workloads[workloadID] = workload
}

func (d *DataStore) calculateWorkloadStatusLocked(workload Workload) string {
	if workload.RunningJobs > 0 {
		return WorkloadRunning
	}

	hasQueued := false
	for _, imageID := range workload.OriginalImages {
		image, ok := d.Images[imageID]
		if !ok {
			continue
		}
		switch image.Status {
		case ImageQueued:
			hasQueued = true
		case ImageRunning:
			return WorkloadRunning
		case ImageCompleted:
			continue
		default:
			hasQueued = true
		}
	}

	if len(workload.OriginalImages) > 0 && !hasQueued && len(workload.FilteredImages) >= len(workload.OriginalImages) {
		return WorkloadCompleted
	}

	return WorkloadScheduling
}

func (d *DataStore) decrementWorkerLocked(workerID int) {
	worker, ok := d.Workers[workerID]
	if !ok {
		return
	}
	if worker.RunningJobs > 0 {
		worker.RunningJobs--
	}
	d.Workers[workerID] = worker
}

func appendIfMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
