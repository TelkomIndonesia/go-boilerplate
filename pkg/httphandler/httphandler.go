package httphandler

import "github.com/telkomindonesia/go-boilerplate/pkg/profile"

type OptFunc func(h *HTTPHandler) error

func WithProfileRepository(pr profile.ProfileRepository) OptFunc {
	return func(h *HTTPHandler) error {
		h.profileRepo = pr
		return nil
	}
}

type HTTPHandler struct {
	profileRepo profile.ProfileRepository
}

func New(opts ...OptFunc) (h *HTTPHandler, err error) {
	h = &HTTPHandler{}
	for _, opt := range opts {
		if err = opt(h); err != nil {
			return
		}
	}

	return
}
