package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"log"
	"math"
	"strconv"
)

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, model.ErrorResponse{Error: model.ErrorBody{Code: code, Message: message}})
}
func repositoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		Error(c, 404, "not_found", "Resource not found")
	case errors.Is(err, repository.ErrConflict):
		Error(c, 409, "conflict", "Resource already exists")
	case errors.Is(err, repository.ErrInvalidReference):
		Error(c, 422, "invalid_reference", "Referenced category or author does not exist")
	case errors.Is(err, repository.ErrValidation):
		Error(c, 422, "validation_error", "Resource fields are invalid")
	default:
		log.Printf("repository operation failed (%T)", err)
		Error(c, 500, "internal_server_error", "Internal server error")
	}
}
func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		Error(c, 400, "invalid_id", "ID must be an unsigned 32-bit integer")
		return 0, false
	}
	return uint(n), true
}
func readInput[T any](c *gin.Context) (T, bool) {
	var input T
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBody+1))
	trimmed := bytes.TrimSpace(body)
	if err != nil || len(body) > maxBody || len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(body) {
		Error(c, 422, "validation_error", "Body must be one valid JSON object (at most 1 MiB)")
		return input, false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		Error(c, 422, "validation_error", "Request contains invalid or unknown fields")
		return input, false
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, 422, "validation_error", "Request fields are invalid")
		return input, false
	}
	return input, true
}

func boundedPage(c *gin.Context) (int, int, bool) {
	page, ok := pagination(c, "page", 1)
	if !ok {
		return 0, 0, false
	}
	limit, ok := pagination(c, "limit", 20)
	if !ok {
		return 0, 0, false
	}
	if limit > 100 {
		limit = 100
	}
	if page-1 > uint64(math.MaxInt)/limit {
		Error(c, 400, "invalid_pagination", "Page is too large")
		return 0, 0, false
	}
	return int(limit), int((page - 1) * limit), true
}
func pagination(c *gin.Context, key string, fallback uint64) (uint64, bool) {
	values, present := c.Request.URL.Query()[key]
	if !present {
		return fallback, true
	}
	if len(values) != 1 {
		Error(c, 400, "invalid_pagination", "Specify each pagination parameter once")
		return 0, false
	}
	n, err := strconv.ParseUint(values[0], 10, 64)
	if err != nil || n == 0 {
		Error(c, 400, "invalid_pagination", "Page and limit must be positive integers")
		return 0, false
	}
	return n, true
}
