package httpserver

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h HTTPServer) setProfileGroup() {
	profile := h.handler.Group("/tenants/:tenantid/profiles")
	h.getProfile(profile)
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

		err = h.profileMgr.ValidateProfile(c.Request().Context(), pr)
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid profile")
		}

		return c.JSON(http.StatusOK, pr)
	})

}
