package echo

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"timonier.systems/lookout/assets"
	"timonier.systems/lookout/pkg/common/handler"
	"timonier.systems/lookout/pkg/repository"
	"timonier.systems/lookout/pkg/service"
	"timonier.systems/lookout/pkg/ui/dgraph"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func NewTemplateRenderer() (*TemplateRenderer, error) {
	templateFS, err := fs.Sub(assets.Assets, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to access embedded templates: %w", err)
	}

	// Define custom template functions
	funcMap := template.FuncMap{
		"mul": func(a, b int) int {
			return a * b
		},
		"reverse": func(slice interface{}) interface{} {
			// Reverse a slice using reflection
			s := reflect.ValueOf(slice)
			if s.Kind() != reflect.Slice {
				return slice
			}
			reversed := make([]interface{}, s.Len())
			for i := 0; i < s.Len(); i++ {
				reversed[s.Len()-1-i] = s.Index(i).Interface()
			}
			return reversed
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &TemplateRenderer{
		templates: tmpl,
	}, nil
}

// HomePage serves the home page
func HomePage(c echo.Context) error {
	data, err := fs.ReadFile(assets.Assets, "templates/index.html")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Unable to load home page")
	}
	return c.Blob(http.StatusOK, "text/html", data)
}

// HealthCheck endpoint for Docker healthchecks and monitoring
func HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "lookout-ui",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// ReadyCheck endpoint for Kubernetes readiness probes
func ReadyCheck(c echo.Context) error {
	// TODO: Add checks for dependencies (Dgraph, etc.)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "ready",
	})
}

func LaunchWebServer() {
	e := echo.New()

	// Hide Echo banner for cleaner logs
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

	// Security middleware
	e.Use(middleware.Secure())
	e.Use(middleware.CORS())

	// Timeouts
	// Extended timeout for operations involving NVD API calls (rate limiting can slow things down)
	// Skip timeout for SSE endpoints (they need to stream indefinitely)
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout: 5 * time.Minute,
		Skipper: func(c echo.Context) bool {
			// Skip timeout for SSE progress endpoints and results pages
			return c.Path() == "/progress/:sessionId" ||
				c.Path() == "/results/:sessionId"
		},
	}))

	// Static files
	staticFS, err := fs.Sub(assets.Assets, "static")
	if err != nil {
		log.Printf("ERROR: Failed to access static files: %v", err)
		log.Printf("Server will continue but static files won't be available")
	} else {
		e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))))
	}

	// Templates
	renderer, err := NewTemplateRenderer()
	if err != nil {
		log.Printf("ERROR: Failed to initialize templates: %v", err)
		log.Printf("Server will continue but templates won't work")
	} else {
		e.Renderer = renderer
	}

	// Health and readiness endpoints
	e.GET("/health", HealthCheck)
	e.GET("/ready", ReadyCheck)
	e.GET("/healthz", HealthCheck) // Kubernetes convention

	// Initialize dependencies for handlers that require them
	clientManager := dgraph.GetGlobalClientManager()
	repo := repository.NewDgraphRepository(clientManager)
	vulnService := service.NewVulnerabilityService(repo)

	deps := &handler.HandlerDependencies{
		VulnService: vulnService,
		Repo:        repo,
	}

	// Application routes
	e.GET("/", HomePage)
	e.POST("/process", handler.ProcessCVE)
	e.POST("/upload", handler.UploadAndProcess)
	e.POST("/upload-cyclonedx-bom", handler.UploadBOMWithProgress) // Async SBOM analysis with SSE
	e.POST("/purl-traversal", handler.PurlTraversal(deps))

	// Progress tracking routes
	e.GET("/progress/:sessionId", handler.ProgressSSE)
	e.GET("/results/:sessionId", handler.GetSBOMResults) // SBOM analysis results

	// Get port from environment or use default
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Starting Echo server on port %s...", port)

	// Start server in a goroutine for graceful shutdown
	go func() {
		if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
			log.Printf("ERROR: Server failed to start: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Printf("ERROR: Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
