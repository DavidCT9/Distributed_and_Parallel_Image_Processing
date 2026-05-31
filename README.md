# Distributed and Parallel Image Processing

## Project description

Distributed and Parallel Image Processing (DPIP) is a Go system that receives image workloads through a REST API and distributes image-filtering jobs to worker nodes through RPC.

The master process starts two services:

- A Gin HTTP API on port `8080` for clients.
- A controller RPC server on port `40901` for workers.

Workers register with the controller, receive the API address and an internal worker token, then wait for jobs. When a client uploads an original image, the controller schedules that image to the least busy worker. The worker downloads the image, applies the requested filter, uploads the filtered image, and the controller updates the workload status.

## How it works

1. A client logs in with `POST /login` and receives a token.
2. The client creates a workload with filter `grayscale` or `blur`.
3. One or more workers connect to the controller.
4. The client uploads original images to the workload.
5. The scheduler chooses a worker using this order:
   - lowest number of running jobs
   - lowest CPU value
   - lowest RAM value
6. The worker processes the image and uploads the filtered output.
7. The client checks the workload status and downloads filtered images.

## Technology used

- Go
- Gin Web Framework
- Go `net/rpc`
- Go standard image libraries
- Python helper scripts in `Tests/` for video frame extraction and stress testing

## Structure

- `cmd/`: master node entry point.
- `API/`: REST API and token authentication.
- `Controller/`: controller state, worker registration, and RPC job dispatch.
- `Scheduler/`: worker selection policy.
- `Worker/`: worker node, image filters, and worker RPC server.
- `Tests/`: helper scripts provided for frame extraction, stress upload, download, and video reconstruction.
- `user-guide.md`: detailed installation and usage guide.

## Main features

- Token-authenticated API endpoints required by the rubric.
- Worker registration with controller through RPC.
- Scheduler for assigning images to workers.
- Worker-side grayscale and blur filtering.
- Workload status tracking: `scheduling`, `running`, `completed`.
- Filtered image download through the API.

## Features for future work

- Add persistent storage so workloads survive server restarts.
- Add a worker heartbeat timeout to mark disconnected workers as unavailable.
- Add a web dashboard for workload progress and worker utilization.
- Add CUDA-based filters for the extra-credit GPU path.
- Add stronger user management instead of demo credentials.

## Quick start

Read [user-guide.md](user-guide.md) for full setup and test instructions.

Basic startup:

```powershell
go run ./cmd
```

In another terminal:

```powershell
go run ./Worker/cmd --controller localhost:40901 --worker-name worker1 --tags cpu,grayscale,blur
```

Demo credentials:

- User: `user`
- Password: `password`

## Members

- Add team member names here before submission.
