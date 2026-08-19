//go:build tools

// Package tools pins build-only dependencies used by go:generate. It is never
// included in the application binary.
package tools

import _ "github.com/urfave/cli/v2"
