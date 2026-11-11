// Package api embeds the ExtContract (OpenAPI 3.0.3) for the contract test.
// The running API does not serve it (D21); it is handed to the IDM team as a file.
package api

import _ "embed"

// Spec is the raw backend-extapi/api/openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
