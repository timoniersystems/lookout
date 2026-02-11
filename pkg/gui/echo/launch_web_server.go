package echo

import (
	"defender/assets"
	"defender/pkg/common/handler"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func NewTemplateRenderer() *TemplateRenderer {
	templateFS, err := fs.Sub(assets.Assets, "templates")
	if err != nil {
		log.Fatal("Failed to access embedded templates:", err)
	}

	tmpl, err := template.New("").ParseFS(templateFS, "*.html")
	if err != nil {
		log.Fatal("Error parsing templates:", err)
	}

	return &TemplateRenderer{
		templates: tmpl,
	}
}

func HomePage(c echo.Context) error {
	data, err := fs.ReadFile(assets.Assets, "templates/index.html")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Unable to load home page")
	}
	return c.Blob(http.StatusOK, "text/html", data)
}

func LaunchWebServer() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	staticFS, err := fs.Sub(assets.Assets, "static")
	if err != nil {
		log.Fatal("Failed to access static files:", err)
	}
	e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))))

	renderer := NewTemplateRenderer()
	e.Renderer = renderer

	e.GET("/", HomePage)
	e.POST("/process", handler.ProcessCVE)
	e.POST("/upload", handler.UploadAndProcess)
	e.POST("/sbom-process", handler.RunTrivyAndProcess)
	e.POST("/upload-cyclonedx-bom", handler.UploadBOMAndInsertData)
	e.POST("/purl-traversal", handler.PurlTraversal)

	log.Println("Starting Echo server...")
	if err := e.Start(":3000"); err != nil {
		log.Fatal("Echo server shut down. Error:", err)
	}
}
