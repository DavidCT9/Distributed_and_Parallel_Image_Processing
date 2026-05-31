package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	controller "github.com/DavidCT9/Image_Filtering_API/Controller"
	"github.com/gin-gonic/gin"
)

// receives the petitions of the clients
type API struct {
	Controller *controller.Controller
	Port       int
	tokens     map[string]string //saves the active tokens: TOKEN -> USER
	tokenMutex sync.RWMutex
}

func NewAPI(c *controller.Controller, port int) *API {
	return &API{
		Controller: c,
		Port:       port,
		tokens:     make(map[string]string),
	}
}

func (a *API) Start() {
	router := gin.Default()

	// public routes
	router.POST("/login", a.login)

	// routes that need a token
	authorized := router.Group("/")
	authorized.Use(a.authMiddleware())
	{
		authorized.DELETE("/logout", a.logout)
		authorized.GET("/status", a.status)
		authorized.POST("/workloads", a.createWorkload)
		authorized.GET("/workloads/:id", a.getWorkload)
		authorized.POST("/images", a.uploadImage)
		authorized.GET("/images/:id", a.downloadImage)
		authorized.GET("/images", a.getImages)

	}

	if err := router.Run(fmt.Sprintf(":%d", a.Port)); err != nil {
		panic(err)
	}
}

// validates that the request has the same token
func (a *API) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")

		if a.Controller.IsWorkerToken(token) {
			c.Set("token", token)
			c.Next()
			return
		}

		a.tokenMutex.RLock()
		_, ok := a.tokens[token]
		a.tokenMutex.RUnlock()

		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		c.Set("token", token)
		c.Next()
	}
}

func (a *API) login(c *gin.Context) {
	user, password, ok := c.Request.BasicAuth()
	if !ok || user != "user" || password != "password" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token := generateToken()

	a.tokenMutex.Lock()
	a.tokens[token] = user
	a.tokenMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "Hello!, welcome to the DPIP System",
		"token":   token,
	})
}

func (a *API) logout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")

	a.tokenMutex.Lock()
	delete(a.tokens, token)
	a.tokenMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"logout_message": "User logged out successfully",
	})
}

func (a *API) status(c *gin.Context) {
	workers := a.Controller.DataStore.GetWorkers()
	workloads := a.Controller.DataStore.GetAllWorkloads()

	c.JSON(http.StatusOK, gin.H{
		"system_name":      "DPIP System",
		"server_time":      time.Now().Format(time.RFC3339),
		"active_workloads": workloads,
		"workers":          workers,
	})
}

func (a *API) createWorkload(c *gin.Context) {
	var body struct {
		Filter string `json:"filter"`
		Name   string `json:"workload_name"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		if errors.Is(err, io.EOF) {
			body.Filter = "grayscale"
			body.Name = fmt.Sprintf("workload_%d", time.Now().UnixNano())
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
	}

	body.Filter = strings.ToLower(strings.TrimSpace(body.Filter))
	body.Name = strings.TrimSpace(body.Name)

	if body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workload_name is required"})
		return
	}
	if body.Filter != "grayscale" && body.Filter != "blur" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filter must be grayscale or blur"})
		return
	}

	workloadID := fmt.Sprintf("%d", time.Now().UnixNano())
	directory := fmt.Sprintf("%s_%s", safeName(body.Name), workloadID)
	workload := controller.Workload{
		ID:             workloadID,
		Name:           body.Name,
		Directory:      directory,
		Filter:         body.Filter,
		Status:         controller.WorkloadScheduling,
		RunningJobs:    0,
		FilteredImages: []string{},
	}

	if err := os.MkdirAll(filepath.Join("images", workload.Directory), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create workload directory"})
		return
	}

	a.Controller.DataStore.AddWorkload(workload)

	c.JSON(http.StatusOK, workloadResponse(workload))
}

// getWorkload regresa los detalles de un workload específico por su ID
func (a *API) getWorkload(c *gin.Context) {
	id := c.Param("id")

	workload, ok := a.Controller.DataStore.GetWorkload(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workload not found"})
		return
	}

	c.JSON(http.StatusOK, workloadResponse(workload))
}

// uploadImage recibe una imagen, la guarda en disco y la registra en el datastore
// uploadImage receives an image, saves it to disk and registers it in the datastore
func (a *API) uploadImage(c *gin.Context) {
	workloadID := c.PostForm("workload_id")
	imageType := strings.ToLower(strings.TrimSpace(c.PostForm("type")))

	if imageType != controller.ImageOriginal && imageType != controller.ImageFiltered {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be original or filtered"})
		return
	}

	// make sure the workload exists before saving the image
	workload, ok := a.Controller.DataStore.GetWorkload(workloadID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workload not found"})
		return
	}

	// get the image file from the request
	file, err := c.FormFile("data")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image not found in request"})
		return
	}

	// generate a unique ID for the image
	imageID := fmt.Sprintf("%d", time.Now().UnixNano())

	// create the workload directory if it doesn't exist
	dir := filepath.Join("images", workload.Directory)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create image directory"})
		return
	}

	// save the image to disk
	path := filepath.Join(dir, imageID+imageExtension(file.Filename))
	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save image"})
		return
	}

	// register the image in the datastore
	image := controller.Image{
		ImageID:    imageID,
		WorkloadID: workloadID,
		Type:       imageType,
		Path:       path,
	}
	a.Controller.DataStore.AddImage(image)

	if imageType == controller.ImageOriginal {
		go a.Controller.SchedulePendingJobs()
	}

	c.JSON(http.StatusOK, gin.H{
		"image_id":    imageID,
		"workload_id": workloadID,
		"type":        imageType,
	})
}

// finds an image on disk and returns it as a file
func (a *API) downloadImage(c *gin.Context) {
	id := c.Param("id")

	image, ok := a.Controller.DataStore.GetImage(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}

	// check that the file actually exists on disk
	if _, err := os.Stat(image.Path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "image file not found"})
		return
	}

	c.File(image.Path)
}

// getImages returns the list of all images registered in the datastore
func (a *API) getImages(c *gin.Context) {
	images := a.Controller.DataStore.GetAllImages()
	c.JSON(http.StatusOK, images)
}

func workloadResponse(workload controller.Workload) gin.H {
	return gin.H{
		"workload_id":     workload.ID,
		"filter":          workload.Filter,
		"workload_name":   workload.Name,
		"status":          workload.Status,
		"running_jobs":    workload.RunningJobs,
		"filtered_images": workload.FilteredImages,
	}
}

func generateToken() string {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func imageExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif":
		return ext
	default:
		return ".png"
	}
}

func safeName(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "workload"
	}
	return result
}
