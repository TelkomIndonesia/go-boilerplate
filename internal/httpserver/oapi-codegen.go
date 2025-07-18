//go:generate go tool github.com/telkomindonesia/oapik/cmd/oapik bundle ../../api/openapi-spec/src/profile.yml ../../api/openapi-spec/profile.yml
//go:generate go tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config oapi-codegen.yml ../../api/openapi-spec/profile.yml
package httpserver

import (
	"github.com/telkomindonesia/go-boilerplate/internal/httpserver/internal/oapi"
)

var _ oapi.StrictServerInterface = oapiServerImplementation{}

type oapiServerImplementation struct {
	h *HTTPServer
}
