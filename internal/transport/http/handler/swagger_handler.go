package handler

import (
	"net/http"
	"strings"

	"github.com/unowned-22/api/internal/docs"
)

// SwaggerHandler serves Swagger UI and the raw OpenAPI spec.
type SwaggerHandler struct{}

// NewSwaggerHandler creates a new SwaggerHandler.
func NewSwaggerHandler() *SwaggerHandler {
	return &SwaggerHandler{}
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>API Documentation</title>
  <link rel="stylesheet"
        href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui.min.css" />
  <style>
    html { box-sizing: border-box; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; padding: 0; }
    .swagger-ui .topbar { display: none; }
  </style>
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui-bundle.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui-standalone-preset.min.js"></script>
<script>
  window.onload = function () {
    SwaggerUIBundle({
      url: "/swagger/openapi.yaml",
      dom_id: "#swagger-ui",
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
      layout: "StandaloneLayout",
      deepLinking: true,
      defaultModelsExpandDepth: 1,
      defaultModelExpandDepth: 1,
    });
  };
</script>
</body>
</html>`

// Index serves the Swagger UI HTML page.
func (h *SwaggerHandler) Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerHTML))
}

// Spec serves the raw OpenAPI YAML specification.
func (h *SwaggerHandler) Spec(w http.ResponseWriter, r *http.Request) {
	if len(docs.Spec) == 0 {
		http.Error(w, "spec not found", http.StatusNotFound)
		return
	}

	// Allow the Swagger UI page (same origin) to fetch the spec.
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docs.Spec)
}

// Redirect sends bare /swagger requests to /swagger/index.html.
func (h *SwaggerHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	target := "/swagger/index.html"
	if q := r.URL.RawQuery; q != "" {
		target += "?" + q
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

// SwaggerAvailableInEnv AvailableInEnv returns true for any environment except "production" / "prod".
// Use this to gate the /swagger routes so the UI is never exposed in prod.
func SwaggerAvailableInEnv(appEnv string) bool {
	env := strings.ToLower(strings.TrimSpace(appEnv))
	return env != "production" && env != "prod"
}
