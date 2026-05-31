package scheduler

import "testing"

func TestSelectWorkerUsesRunningJobsThenCPUThenRAM(t *testing.T) {
	workers := []WorkerCandidate{
		{ID: 1, RunningJobs: 2, CPU: 10, RAM: 10},
		{ID: 2, RunningJobs: 1, CPU: 80, RAM: 80},
		{ID: 3, RunningJobs: 1, CPU: 20, RAM: 90},
		{ID: 4, RunningJobs: 1, CPU: 20, RAM: 30},
	}

	selected, ok := SelectWorker(workers)
	if !ok {
		t.Fatal("expected a worker to be selected")
	}
	if selected.ID != 4 {
		t.Fatalf("expected worker 4, got %d", selected.ID)
	}
}

func TestSelectWorkerWithNoWorkers(t *testing.T) {
	if _, ok := SelectWorker(nil); ok {
		t.Fatal("expected no worker to be selected")
	}
}
