// Package api holds the OpenAPI contract of backend-exq (export jobs).
// Only tests import it; the spec is not served.
package api

import _ "embed"

// Spec is backend-exq/api/openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
