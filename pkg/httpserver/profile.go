package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

func (h *HTTPServer) registerProfileGroup() *HTTPServer {
	profile := h.handler.Group("/tenants/:tenantid/profiles")
	h.getProfile(profile).
		putProfile(profile)
	return h
}

func (h *HTTPServer) putProfile(g *echo.Group) *HTTPServer {
	g.PUT("/:id", func(c echo.Context) error {
		tid, err := uuid.Parse(c.Param("tenantid"))
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid tenant id")
		}
		pid, err := uuid.Parse(c.Param("id"))
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid profile id")
		}

		var pr *profile.Profile
		err = json.NewDecoder(c.Request().Body).Decode(&pr)
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid profile")
		}
		pr.TenantID = tid
		pr.ID = pid

		err = h.profileRepo.StoreProfile(c.Request().Context(), pr)
		if err != nil {
			return c.String(http.StatusInternalServerError, "invalid profile")
		}

		h.logger.Info("profile_stored", log.TraceContext("trace-id", c.Request().Context()), log.Any("profile", pr))
		return c.String(http.StatusCreated, "profile stored")
	})
	return h
}

func (h *HTTPServer) getProfile(g *echo.Group) *HTTPServer {
	g.GET("/:id", func(c echo.Context) error {
		tid, err := uuid.Parse(c.Param("tenantid"))
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid tenant id")
		}
		pid, err := uuid.Parse(c.Param("id"))
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid profile id")
		}

		pr, err := h.profileRepo.FetchProfile(c.Request().Context(), tid, pid)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		if pr == nil {
			return c.String(http.StatusNotFound, "profile not found")
		}

		if c.QueryParam("validate") == "true" {
			err = h.profileMgr.ValidateProfile(c.Request().Context(), pr)
			if err != nil {
				return c.String(http.StatusBadRequest, "invalid profile")
			}
		}

		return c.JSON(http.StatusOK, pr)
	})
	return h
}

func (h *HTTPServer) registerTenantPassthrough() *HTTPServer {
	h.handler.GET("/tenants/:tenantid", func(c echo.Context) error {
		tid, err := uuid.Parse(c.Param("tenantid"))
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid tenant id")
		}
		t, err := h.tenantRepo.FetchTenant(c.Request().Context(), tid)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, t)
	})
	return h
}
