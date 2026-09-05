package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorInfo `json:"error,omitempty"`
	Meta    *Meta      `json:"meta,omitempty"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// OK sends a success response.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// Created sends a success response for a newly created resource.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Data:    data,
	})
}

// Page sends a success response alongside pagination metadata.
func Page(c *gin.Context, data any, page, perPage, total int) {
	totalPages := 0

	if perPage > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

func Fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, Response{
		Success: false,
		Error:   &ErrorInfo{Code: code, Message: message},
	})
}

// FailWithDetails sends an error response carrying a machine readable list of
// reasons, used for schema validation failures.
func FailWithDetails(c *gin.Context, status int, code, message string, details any) {
	c.JSON(status, Response{
		Success: false,
		Error:   &ErrorInfo{Code: code, Message: message, Details: details},
	})
}

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("panic: %v\n", recovered)

		Fail(c, http.StatusInternalServerError, "internal_error", "internal server error")

		c.Abort()
	})
}

// NotFound returns a JSON 404 envelope for unmatched routes.
func NotFound(c *gin.Context) {
	Fail(c, http.StatusNotFound, "not_found", "resource not found")
}
