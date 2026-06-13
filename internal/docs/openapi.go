// Package docs embeds the OpenAPI specification so it can be served at runtime
// without relying on the filesystem layout of the deployment environment.
package docs

import _ "embed"

// Spec is the raw OpenAPI 3.0 YAML document.
//
//go:embed openapi.yaml
var Spec []byte
