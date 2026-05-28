package api

import (
	"fmt"
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

	authorized := router.Group("/")
	authorized.Use(a.authMiddleware())
	{
		authorized.DELETE("/logout", a.logout)
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
