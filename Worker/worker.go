package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"net/rpc"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Worker is the node that receives image jobs from the controller and applies
// the configured filter.
type Worker struct {
	Name           string
	ControllerAddr string
	APIAddress     string
	Token          string
	Tags           []string
	ID             int
	Address        string
	RPCClient      *rpc.Client

	mutex       sync.Mutex
	runningJobs int
}

type RegisterArgs struct {
	Name    string
	Address string
	Tags    []string
}

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

type WorkerRPC struct {
	worker *Worker
}

// NewWorker creates a new worker with its name and controller address.
func NewWorker(name, controllerAddr string, tags []string) *Worker {
	return &Worker{
		Name:           name,
		ControllerAddr: controllerAddr,
		Tags:           tags,
	}
}

// Start exposes the worker RPC server, connects to the controller, and
// registers this worker as available for scheduling.
func (w *Worker) Start() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Error starting worker RPC server: %v", err)
	}
	w.Address = listener.Addr().String()

	server := rpc.NewServer()
	if err := server.RegisterName("WorkerRPC", &WorkerRPC{worker: w}); err != nil {
		log.Fatalf("Error registering worker RPC service: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Println("Worker RPC connection error:", err)
				continue
			}
			go server.ServeConn(conn)
		}
	}()

	client, err := rpc.Dial("tcp", w.ControllerAddr)
	if err != nil {
		log.Fatalf("Error connecting to controller: %v", err)
	}
	w.RPCClient = client

	fmt.Printf("Connected to controller at %s\n", w.ControllerAddr)

	args := RegisterArgs{
		Name:    w.Name,
		Address: w.Address,
		Tags:    w.Tags,
	}
	reply := RegisterReply{}

	err = w.RPCClient.Call("RPCController.RegisterWorker", args, &reply)
	if err != nil {
		log.Fatalf("Error registering worker: %v", err)
	}

	w.ID = reply.WorkerID
	w.APIAddress = strings.TrimRight(reply.APIAddress, "/")
	w.Token = reply.Token

	fmt.Printf("Worker %s registered with ID: %d\n", w.Name, w.ID)
	fmt.Printf("Worker RPC address: %s\n", w.Address)
	fmt.Printf("API address: %s\n", w.APIAddress)

	w.reportStats()
	select {}
}

func (r *WorkerRPC) ProcessImageJob(args ProcessImageJobArgs, reply *ProcessImageJobReply) error {
	filter := strings.ToLower(strings.TrimSpace(args.Filter))
	if filter != "grayscale" && filter != "blur" {
		reply.Success = false
		reply.Error = "unsupported filter"
		return nil
	}

	r.worker.beginJob()
	defer r.worker.endJob()

	apiAddress := strings.TrimRight(args.APIAddress, "/")
	token := args.Token
	if apiAddress == "" {
		apiAddress = r.worker.APIAddress
	}
	if token == "" {
		token = r.worker.Token
	}

	original, err := downloadImage(apiAddress, token, args.ImageID)
	if err != nil {
		reply.Success = false
		reply.Error = err.Error()
		return nil
	}

	filtered, err := applyFilter(original, filter)
	if err != nil {
		reply.Success = false
		reply.Error = err.Error()
		return nil
	}

	filteredID, err := uploadFilteredImage(apiAddress, token, args.WorkloadID, filtered)
	if err != nil {
		reply.Success = false
		reply.Error = err.Error()
		return nil
	}

	reply.Success = true
	reply.FilteredImageID = filteredID
	return nil
}

func (w *Worker) beginJob() {
	w.mutex.Lock()
	w.runningJobs++
	w.mutex.Unlock()
	w.reportStats()
}

func (w *Worker) endJob() {
	w.mutex.Lock()
	if w.runningJobs > 0 {
		w.runningJobs--
	}
	w.mutex.Unlock()
	w.reportStats()
}

func (w *Worker) currentJobs() int {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.runningJobs
}

func (w *Worker) reportStats() {
	if w.RPCClient == nil || w.ID < 0 {
		return
	}

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	runningJobs := w.currentJobs()
	cpu := math.Min(100, float64(runningJobs)*25)
	ram := float64(stats.Alloc) / (1024 * 1024)

	args := WorkerStatsArgs{
		WorkerID:    w.ID,
		CPU:         cpu,
		RAM:         ram,
		RunningJobs: runningJobs,
	}
	reply := WorkerStatsReply{}
	if err := w.RPCClient.Call("RPCController.UpdateWorkerStats", args, &reply); err != nil {
		log.Printf("Error reporting worker stats: %v", err)
	}
}

func downloadImage(apiAddress, token, imageID string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/images/%s", apiAddress, imageID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func uploadFilteredImage(apiAddress, token, workloadID string, data []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("workload_id", workloadID); err != nil {
		return "", err
	}
	if err := writer.WriteField("type", "filtered"); err != nil {
		return "", err
	}

	part, err := writer.CreateFormFile("data", "filtered.png")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, apiAddress+"/images", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		ImageID string `json:"image_id"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if parsed.ImageID == "" {
		return "", errors.New("API did not return image_id")
	}

	return parsed.ImageID, nil
}

func applyFilter(input []byte, filter string) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}

	var output *image.RGBA
	switch filter {
	case "grayscale":
		output = grayscale(img)
	case "blur":
		output = blur(img)
	default:
		return nil, fmt.Errorf("unsupported filter %q", filter)
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, output); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func grayscale(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			gray := uint8(((r * 299) + (g * 587) + (b * 114) + 500000) / 1000000)
			out.Set(x, y, color.RGBA{R: gray, G: gray, B: gray, A: uint8(a >> 8)})
		}
	}

	return out
}

func blur(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var red, green, blue, alpha uint32
			var count uint32

			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx := x + dx
					ny := y + dy
					if nx < bounds.Min.X || nx >= bounds.Max.X || ny < bounds.Min.Y || ny >= bounds.Max.Y {
						continue
					}
					r, g, b, a := img.At(nx, ny).RGBA()
					red += r
					green += g
					blue += b
					alpha += a
					count++
				}
			}

			out.Set(x, y, color.RGBA{
				R: uint8((red / count) >> 8),
				G: uint8((green / count) >> 8),
				B: uint8((blue / count) >> 8),
				A: uint8((alpha / count) >> 8),
			})
		}
	}

	return out
}
