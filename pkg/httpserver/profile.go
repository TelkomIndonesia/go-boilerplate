package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
)

func (h HTTPServer) setProfileGroup() {
	profile := h.handler.Group("/tenants/:tenantid/profiles")
	h.getProfile(profile)
	h.putProfile(profile)
}

func (h HTTPServer) putProfile(g *echo.Group) {
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

		return c.String(http.StatusCreated, "profile stored")
	})
}
func (h HTTPServer) getProfile(g *echo.Group) {
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

}

func (h HTTPServer) tenantPassthrough() {
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
}
