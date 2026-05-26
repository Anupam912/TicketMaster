package handlers

import (
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SwaggerHandler serves custom Swagger UI with bearer token interceptor.
type SwaggerHandler struct{}

// NewSwaggerHandler creates a new swagger handler.
func NewSwaggerHandler() *SwaggerHandler {
	return &SwaggerHandler{}
}

const swaggerIndexTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Event Ticketing API - Swagger UI</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/swagger/doc.json",
                dom_id: '#swagger-ui',
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                layout: "StandaloneLayout",
                persistAuthorization: true,
                requestInterceptor: function(req) {
                    if (req.headers && req.headers.Authorization) {
                        var token = req.headers.Authorization;
                        if (token && !token.startsWith('Bearer ')) {
                            req.headers.Authorization = 'Bearer ' + token;
                        }
                    }
                    return req;
                }
            });
        };
    </script>
</body>
</html>`

// ServeSwaggerUI serves the custom Swagger UI page.
func (h *SwaggerHandler) ServeSwaggerUI(c *gin.Context) {
	tmpl, err := template.New("swagger").Parse(swaggerIndexTemplate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load swagger ui"})
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render swagger ui"})
	}
}
