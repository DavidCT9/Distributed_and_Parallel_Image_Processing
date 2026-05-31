package scheduler

// WorkerCandidate is the minimal worker state needed by the scheduling policy.
type WorkerCandidate struct {
	ID          int
	RunningJobs int
	CPU         float64
	RAM         float64
}

// SelectWorker chooses the least busy worker. It keeps the policy intentionally
// simple for the course project: fewer running jobs first, then lower CPU, then
// lower RAM as tie-breakers.
func SelectWorker(workers []WorkerCandidate) (WorkerCandidate, bool) {
	if len(workers) == 0 {
		return WorkerCandidate{}, false
	}

	best := workers[0]
	for _, worker := range workers[1:] {
		if worker.RunningJobs < best.RunningJobs {
			best = worker
			continue
		}
		if worker.RunningJobs == best.RunningJobs && worker.CPU < best.CPU {
			best = worker
			continue
		}
		if worker.RunningJobs == best.RunningJobs && worker.CPU == best.CPU && worker.RAM < best.RAM {
			best = worker
		}
	}

	return best, true
}
