package web

import (
	_ "embed"
	"net/http"

	"colosscious.com/gitfetcher/scheduler"
	"github.com/gin-gonic/gin"
)

//go:embed templates/index.html
var indexHTML string

type Handler struct {
	scheduler *scheduler.Scheduler
}

func NewHandler(s *scheduler.Scheduler) *Handler {
	return &Handler{
		scheduler: s,
	}
}

// SetupRoutes configures all HTTP routes
func (h *Handler) SetupRoutes(r *gin.Engine) {
	r.GET("/", h.handleIndex)
	r.GET("/api/status", h.handleStatus)
	r.POST("/api/fetch/:name", h.handleManualFetch)
}

// handleIndex serves the main HTML page
func (h *Handler) handleIndex(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, indexHTML)
}

// handleStatus returns JSON status of all repositories
func (h *Handler) handleStatus(c *gin.Context) {
	status := h.scheduler.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"repos": status,
	})
}

// handleManualFetch triggers a manual fetch for a specific repository
func (h *Handler) handleManualFetch(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "repository name is required",
		})
		return
	}

	if err := h.scheduler.ManualFetch(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "fetch triggered for " + name,
	})
}
