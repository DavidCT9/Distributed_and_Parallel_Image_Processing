# User Guide

This guide explains how to install, run, and test the Distributed and Parallel Image Processing system.

## Requirements

- Go 1.25.7 or newer.
- Python 3 if you want to use the scripts in `Tests/`.
- Python packages from `Tests/requirements.txt` if you want to extract frames or run the stress test.

Install Python test dependencies:

```powershell
pip install -r Tests\requirements.txt
```

## Running the system

Open one terminal for the master node:

```powershell
go run ./cmd
```

The master starts:

- API: `http://localhost:8080`
- Controller RPC: `localhost:40901`

Open one or more additional terminals for workers:

```powershell
go run ./Worker/cmd --controller localhost:40901 --worker-name worker1 --tags cpu,grayscale,blur
```

Example with a second worker:

```powershell
go run ./Worker/cmd --controller localhost:40901 --worker-name worker2 --tags cpu,grayscale,blur
```

Workers print their assigned ID, API address, and RPC address after connecting.

## Authentication

The demo credentials are:

- User: `user`
- Password: `password`

Login:

```powershell
curl.exe -u user:password -X POST http://localhost:8080/login
```

Copy the `token` value from the JSON response and use it as `<TOKEN>` in the next commands.

Logout:

```powershell
curl.exe -X DELETE -H "Authorization: Bearer <TOKEN>" http://localhost:8080/logout
```

## API workflow

### Check system status

```powershell
curl.exe -H "Authorization: Bearer <TOKEN>" http://localhost:8080/status
```

The response includes:

- `system_name`
- `server_time`
- `active_workloads`
- `workers`

### Create a workload

Supported filters:

- `grayscale`
- `blur`

```powershell
curl.exe -X POST http://localhost:8080/workloads `
  -H "Authorization: Bearer <TOKEN>" `
  -H "Content-Type: application/json" `
  -d "{\"filter\":\"grayscale\",\"workload_name\":\"demo\"}"
```

Possible error:
If the previous command didnt worked (you are probably in powershell )use the following:
```powershell
curl.exe -X POST http://localhost:8080/workloads -H "Authorization: Bearer <TOKEN>"
-H "Content-Type: application/json" -d '{\"filter\":\"grayscale\",\"workload_name\":\"demo\"}'
```

Save the returned `workload_id`.

For compatibility with the provided test guide, `POST /workloads` with an empty body also creates a default grayscale workload.

### Upload an original image

```powershell
curl.exe -X POST http://localhost:8080/images `
  -H "Authorization: Bearer <TOKEN>" `
  -F "data=@C:\path\to\image.png" `
  -F "workload_id=<WORKLOAD_ID>" `
  -F "type=original"
```

When an original image is uploaded, the controller queues it and schedules it to a worker. If no workers are connected, the workload remains in `scheduling` until a worker registers.

### Check workload progress

```powershell
curl.exe -H "Authorization: Bearer <TOKEN>" http://localhost:8080/workloads/<WORKLOAD_ID>
```

Workload statuses:

- `scheduling`: waiting for images or workers.
- `running`: one or more images are being processed.
- `completed`: all uploaded original images have filtered outputs.

The `filtered_images` array contains the IDs that can be downloaded.

### Download a filtered image

```powershell
curl.exe -H "Authorization: Bearer <TOKEN>" `
  http://localhost:8080/images/<FILTERED_IMAGE_ID> `
  --output filtered.png
```

## Using the provided test scripts

The `Tests/` folder contains the helper scripts that can be used to test the system with many frames.

### Extract video frames

Download a sample video such as Big Buck Bunny and put it in the root file, then extract frames:

```powershell
python Tests\video_utils_windows.py -action extract big_buck_bunny_720p_stereo.avi frames
```

This creates PNG files in the `frames/` directory.

### Push frames to the API

Create a workload and save its `workload_id`, then run:

```powershell
python Tests\stress_test.py -action push -workload-id <WORKLOAD_ID> -token <TOKEN> -frames-path frames
```

The script uploads every frame as an original image.

### Pull filtered frames

After the workload is completed:

```powershell
python Tests\stress_test.py -action pull -workload-id <WORKLOAD_ID> -image-type filtered -token <TOKEN> -frames-path filtered-frames
```

### Join filtered frames into a video

```powershell
python Tests\video_utils_windows.py -action join filtered.mp4 filtered-frames
```

The generated video can be submitted for the extra video evidence points.

## Linux or macOS notes

Use the same Go commands. For the video helper, use the Linux script:

```bash
python3 Tests/video_utils_linux.py -action extract big_buck_bunny_720p_stereo.avi frames
python3 Tests/video_utils_linux.py -action join filtered.mp4 filtered-frames
```

Use `curl` instead of `curl.exe`.

## Troubleshooting

### `go test` or `go run` tries to download Go 1.25.7

The project declares Go 1.25.7 in `go.mod`. Install Go 1.25.7 or allow the Go toolchain download.

### Login returns `invalid credentials`

Use the demo credentials exactly:

```text
user:password
```

### API returns `unauthorized`

Make sure the header is:

```text
Authorization: Bearer <TOKEN>
```

Do not include quotes around the token.

### Workload stays in `scheduling`

Start at least one worker:

```powershell
go run ./Worker/cmd --controller localhost:40901 --worker-name worker1 --tags cpu,grayscale,blur
```

The controller will schedule queued images after the worker registers.

### No filtered images appear

Check:

- The uploaded image uses `type=original`.
- The workload filter is `grayscale` or `blur`.
- A worker is connected.
- The worker terminal does not show download, decode, or upload errors.

### `stress_test.py` downloads no images

Check the workload details:

```powershell
curl.exe -H "Authorization: Bearer <TOKEN>" http://localhost:8080/workloads/<WORKLOAD_ID>
```

The `filtered_images` list should contain at least one image ID before pulling.
