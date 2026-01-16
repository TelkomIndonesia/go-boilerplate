package httpclient

import (
	"github.com/telkomindonesia/go-boilerplate/pkg/httpx"
)

// Deprecated: use httpx.Client
type HTTPClient = httpx.Client

var New = httpx.NewClient

type OptFunc = httpx.ClientOptFunc
type Dialer = httpx.Dialer

var WithDialTLS = httpx.ClientWithDialTLS
var WithDial = httpx.ClientWithDial
