
package server

import (
	"embed"
	"net/http"
	"strings"
	"github.com/labstack/echo/v4"
)

// ServeAsset serves static files from the embedded web/dist/assets directory
func (h *UIHandler) ServeAsset(c echo.Context) error {
	assetPath := c.Request().URL.Path
	// Remove /ui prefix
	if strings.HasPrefix(assetPath, "/ui") {
		assetPath = strings.TrimPrefix(assetPath, "/ui")
	}
	// Remove leading slash
	assetPath = strings.TrimPrefix(assetPath, "/")
	// Prepend web/dist/ to get the correct path in the embed
	assetPath = "web/dist/" + assetPath
	data, err := h.uiAssets.ReadFile(assetPath)
	if err != nil {
		return c.String(http.StatusNotFound, "Asset not found")
	}
	contentType := getContentType(assetPath)
	c.Response().Header().Set("Content-Type", contentType)
	return c.Blob(http.StatusOK, contentType, data)
}

type UIHandler struct {
	uiAssets embed.FS
}

func NewUIHandler(assets embed.FS) *UIHandler {
	return &UIHandler{
		uiAssets: assets,
	}
}

func (h *UIHandler) ServeUI(c echo.Context) error {
	path := c.Request().URL.Path
	
	// Remove /ui prefix if present
	if strings.HasPrefix(path, "/ui") {
		path = strings.TrimPrefix(path, "/ui")
	}
	
	// Default to index.html for SPA routing
	if path == "" || path == "/" {
		path = "web/dist/index.html"
	} else {
		path = "web/dist" + path
	}
	
	// Try to serve the requested file
	data, err := h.uiAssets.ReadFile(path)
	if err != nil {
		// If file not found, serve index.html for SPA routing
		data, err = h.uiAssets.ReadFile("web/dist/index.html")
		if err != nil {
			return c.String(http.StatusNotFound, "UI not found")
		}
		path = "web/dist/index.html"
	}
	
	// Set appropriate content type
	contentType := getContentType(path)
	c.Response().Header().Set("Content-Type", contentType)
	
	return c.Blob(http.StatusOK, contentType, data)
}

func getContentType(path string) string {
	if strings.HasSuffix(path, ".html") {
		return "text/html"
	} else if strings.HasSuffix(path, ".css") {
		return "text/css"
	} else if strings.HasSuffix(path, ".js") {
		return "application/javascript"
	} else if strings.HasSuffix(path, ".json") {
		return "application/json"
	}
	return "application/octet-stream"
}