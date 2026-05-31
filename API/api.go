package api

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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

	router.Run(fmt.Sprintf(":%d", a.Port))
}

// validates that the request has the same token
func (a *API) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")

		a.tokenMutex.RLock()
		_, ok := a.tokens[token]
		a.tokenMutex.RUnlock()

		if !ok {
			c.JSON(401, gin.H{"error": "unauthorized"})
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
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	token := fmt.Sprintf("%d", time.Now().UnixNano())

	a.tokenMutex.Lock()
	a.tokens[token] = user
	a.tokenMutex.Unlock()

	c.JSON(200, gin.H{
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

	c.JSON(200, gin.H{
		"logout_message": "User logged out successfully",
	})
}

func (a *API) status(c *gin.Context) {
	workers := a.Controller.DataStore.GetWorkers()
	workloads := a.Controller.DataStore.GetAllWorkloads()

	c.JSON(200, gin.H{
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
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	workload := controller.Workload{
		ID:             fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:           body.Name,
		Filter:         body.Filter,
		Status:         "scheduling",
		RunningJobs:    0,
		FilteredImages: []string{},
	}

	a.Controller.DataStore.AddWorkload(workload)

	c.JSON(200, gin.H{
		"workload_id":     workload.ID,
		"filter":          workload.Filter,
		"workload_name":   workload.Name,
		"status":          workload.Status,
		"running_jobs":    workload.RunningJobs,
		"filtered_images": workload.FilteredImages,
	})
}

// getWorkload regresa los detalles de un workload específico por su ID
func (a *API) getWorkload(c *gin.Context) {
	id := c.Param("id")

	workload := a.Controller.DataStore.GetWorkload(id)
	if workload == nil {
		c.JSON(404, gin.H{"error": "workload not found"})
		return
	}

	c.JSON(200, gin.H{
		"workload_id":     workload.ID,
		"filter":          workload.Filter,
		"workload_name":   workload.Name,
		"status":          workload.Status,
		"running_jobs":    workload.RunningJobs,
		"filtered_images": workload.FilteredImages,
	})
}

// uploadImage recibe una imagen, la guarda en disco y la registra en el datastore
// uploadImage receives an image, saves it to disk and registers it in the datastore
func (a *API) uploadImage(c *gin.Context) {
	workloadID := c.PostForm("workload_id")
	imageType := c.PostForm("type")

	// make sure the workload exists before saving the image
	workload := a.Controller.DataStore.GetWorkload(workloadID)
	if workload == nil {
		c.JSON(404, gin.H{"error": "workload not found"})
		return
	}

	// get the image file from the request
	file, err := c.FormFile("data")
	if err != nil {
		c.JSON(400, gin.H{"error": "image not found in request"})
		return
	}

	// generate a unique ID for the image
	imageID := fmt.Sprintf("%d", time.Now().UnixNano())

	// create the workload directory if it doesn't exist
	dir := fmt.Sprintf("images/%s", workloadID)
	os.MkdirAll(dir, os.ModePerm)

	// save the image to disk
	path := fmt.Sprintf("%s/%s.png", dir, imageID)
	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(500, gin.H{"error": "could not save image"})
		return
	}

	// register the image in the datastore
	image := controller.Image{
		ImageID:     imageID,
		WorkloadID:  workloadID,
		TypeOfImage: imageType,
	}
	a.Controller.DataStore.AddImage(image)

	c.JSON(200, gin.H{
		"image_id":    imageID,
		"workload_id": workloadID,
		"type":        imageType,
	})
}

// finds an image on disk and returns it as a file
func (a *API) downloadImage(c *gin.Context) {
	id := c.Param("id")

	image := a.Controller.DataStore.GetImage(id)
	if image == nil {
		c.JSON(404, gin.H{"error": "image not found"})
		return
	}

	// build the path where the image is stored on disk
	path := fmt.Sprintf("images/%s/%s.png", image.WorkloadID, image.ImageID)

	// check that the file actually exists on disk
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "image file not found"})
		return
	}

	c.File(path)
}

// getImages returns the list of all images registered in the datastore
func (a *API) getImages(c *gin.Context) {
	images := a.Controller.DataStore.GetAllImages()
	c.JSON(200, images)
}
