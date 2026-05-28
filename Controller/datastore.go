package controller

import (
	"sync"
)

type Worker struct {
	ID          int
	Name        string
	Adress      string
	CPU         float64
	RAM         float64
	RunningJobs int
}

type Workload struct {
	ID             string
	Name           string
	Filter         string
	Status         string
	RunningJobs    int
	FilteredImages []string
}

type Image struct {
	ImageID     string
	WorkloadID  string
	TypeOfImage string
}

// saves the global state of the system
type DataStore struct {
	Workers   []Worker
	Workloads []Workload
	Images    []Image
	mutex     sync.RWMutex
}

func NewDataStore() *DataStore {
	return &DataStore{
		Workers:   []Worker{},
		Workloads: []Workload{},
		Images:    []Image{},
	}
}

// add either a worker, workload, image to the data store
func (d *DataStore) AddWorker(w Worker) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Workers = append(d.Workers, w)
}

func (d *DataStore) AddWorkload(w Workload) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Workloads = append(d.Workloads, w)
}

func (d *DataStore) AddImage(i Image) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Images = append(d.Images, i)
}

// gets all the workers in datastore
func (d *DataStore) GetWorkers() []Worker {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.Workers
}

// returns a workload, image by its id
func (d *DataStore) GetWorkload(id string) *Workload {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	for i, w := range d.Workloads {
		if w.ID == id {
			return &d.Workloads[i]
		}
	}
	return nil
}

func (d *DataStore) GetImage(id string) *Image {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	for i, img := range d.Images {
		if img.ImageID == id {
			return &d.Images[i]
		}
	}
	return nil
}
