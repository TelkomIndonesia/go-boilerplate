//go:generate go run github.com/telkomindonesia/openapi-utils/cmd/merge ../../api/openapi-spec/src/profile.yml ../../api/openapi-spec/profile.yml
//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config oapi-codegen.yml ../../api/openapi-spec/profile.yml
package httpserver

import (
	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver/internal/oapi"
)

var _ oapi.StrictServerInterface = oapiServerImplementation{}

type oapiServerImplementation struct {
	h *HTTPServer
}
