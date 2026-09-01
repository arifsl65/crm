// Package main provides smoke tests for the server endpoints.
// Fix #34: Updated CORS tests to use middleware.DynamicCORS instead of removed corsMiddleware.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/accountant-crm/go-backend/internal/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealthHandler(t *testing.T) {
	router := gin.New()
	router.GET("/health", healthHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

func TestNotImplementedHandler(t *testing.T) {
	router := gin.New()
	router.GET("/test", notImplementedHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"not_implemented"`)
}

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	cfg := middleware.CORSConfig{
		StaticOrigins:    []string{"http://localhost:3000", "https://app.example.com"},
		AllowCredentials: true,
		DB:               nil, // No DB for static-only testing
	}

	router := gin.New()
	router.Use(middleware.DynamicCORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Test allowed origin
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
}

func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	cfg := middleware.CORSConfig{
		StaticOrigins:    []string{"http://localhost:3000"},
		AllowCredentials: true,
		DB:               nil,
	}

	router := gin.New()
	router.Use(middleware.DynamicCORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Test disallowed origin
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://malicious.com")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_OptionsRequest(t *testing.T) {
	cfg := middleware.CORSConfig{
		StaticOrigins:    []string{"http://localhost:3000"},
		AllowCredentials: true,
		DB:               nil,
	}

	router := gin.New()
	router.Use(middleware.DynamicCORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Test preflight request
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
}
