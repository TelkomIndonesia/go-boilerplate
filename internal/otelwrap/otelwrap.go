package otelwrap

import (
	"github.com/telkomindonesia/go-boilerplate/internal/profile"
	"go.opentelemetry.io/otel"
)

var Tracer = otel.Tracer("otelwrap")

//go:generate go run github.com/QuangTung97/otelwrap --out profile-repository.go . profile.ProfileRepository
var _ profile.ProfileRepository

//go:generate go run github.com/QuangTung97/otelwrap --out tenant-repository.go . profile.TenantRepository
var _ profile.TenantRepository
