// Package api holds the OpenAPI contract of the ATM backend (sdlc/openapi-contract).
// Only tests import it; the spec is not served.
package api

import _ "embed"

// Spec is backend/api/openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
