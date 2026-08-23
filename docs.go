package main

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec []byte

//go:embed assets/swagger-ui.css
var swaggerCSS []byte

//go:embed assets/swagger-ui-bundle.js
var swaggerJS []byte

const docsHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>No Wrong Door — Unified Resident API</title>
  <link rel="stylesheet" href="/docs/swagger-ui.css">
  <style>
    body {
      margin: 0;
      background: #fafafa;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    }
    .topbar-header {
      background: #1f2937;
      color: #f9fafb;
      padding: 14px 24px;
      font-size: 16px;
      font-weight: 600;
      display: flex;
      align-items: center;
      justify-content: space-between;
      border-bottom: 2px solid #374151;
    }
    .topbar-badge {
      background: #2563eb;
      color: #ffffff;
      font-size: 12px;
      font-weight: 500;
      padding: 3px 10px;
      border-radius: 9999px;
    }
    .swagger-ui .wrapper {
      max-width: 1100px;
      margin: 0 auto;
      padding: 0 16px;
    }
    .swagger-ui .info {
      margin: 24px 0 16px 0;
    }
    .swagger-ui .info .title {
      font-size: 26px;
      color: #111827;
    }
  </style>
</head>
<body>
  <div class="topbar-header">
    <span>No Wrong Door — Unified Resident Integration Service</span>
    <span class="topbar-badge">API Documentation</span>
  </div>
  <div id="swagger-ui"></div>
  <script src="/docs/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
        deepLinking: true,
        displayRequestDuration: true,
        docExpansion: "list",
        defaultModelsExpandDepth: -1,
        filter: true,
        presets: [
          SwaggerUIBundle.presets.apis
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>
`

func serveDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(docsHTML))
}

func serveOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(openAPISpec)
}

func serveSwaggerCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write(swaggerCSS)
}

func serveSwaggerJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write(swaggerJS)
}
