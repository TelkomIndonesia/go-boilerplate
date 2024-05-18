//go:build tools
// +build tools

package tools

import (
	_ "github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen"
	_ "github.com/sqlc-dev/sqlc/cmd/sqlc"
	_ "github.com/telkomindonesia/openapi-utils/cmd/bundle"
)
