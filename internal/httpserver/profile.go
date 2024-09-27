package httpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/internal/httpserver/internal/oapi"
	"github.com/telkomindonesia/go-boilerplate/internal/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

// GetProfile implements oapi.StrictServerInterface.
func (s oapiServerImplementation) GetProfile(ctx context.Context, request oapi.GetProfileRequestObject) (oapi.GetProfileResponseObject, error) {
	pr, err := s.h.profileRepo.FetchProfile(ctx, request.TenantId, request.ProfileId)
	if err != nil {
		err := fmt.Errorf("failed to fetch profile: %w", err)
		s.h.logger.WithTrace(ctx).Error("failed to fetch repo", log.Error("error", err))
		return oapi.GetProfile500JSONResponse{Message: err.Error()}, nil
	}
	if pr == nil {
		return oapi.GetProfile404JSONResponse{Message: "profile not found"}, nil
	}

	return oapi.GetProfile200JSONResponse{
		Id:       pr.ID,
		TenantId: pr.TenantID,
		Name:     pr.Name,
		Nin:      pr.NIN,
		Email:    pr.Email,
		Dob:      pr.DOB,
		Phone:    pr.Phone,
	}, nil
}

// PostProfile implements oapi.StrictServerInterface.
func (s oapiServerImplementation) PostProfile(ctx context.Context, request oapi.PostProfileRequestObject) (oapi.PostProfileResponseObject, error) {
	id, err := uuid.NewV7()
	if err != nil {
		err := fmt.Errorf("failed to create id: %w", err)
		s.h.logger.WithTrace(ctx).Error("failed to post profile", log.Error("error", err))
		return oapi.PostProfile400JSONResponse{Message: err.Error()}, nil
	}

	pr := &profile.Profile{
		ID:       id,
		TenantID: request.TenantId,
		NIN:      request.Body.Nin,
		Name:     request.Body.Name,
		Email:    request.Body.Email,
		Phone:    request.Body.Phone,
		DOB:      request.Body.Dob,
	}

	if request.Params.Validate != nil && *request.Params.Validate {
		err = s.h.profileMgr.ValidateProfile(ctx, pr)
		if err != nil {
			err := fmt.Errorf("failed to validate profie: %w", err)
			s.h.logger.WithTrace(ctx).Error("failed to post profile", log.Error("error", err))
			return oapi.PostProfile400JSONResponse{Message: err.Error()}, nil
		}
	}

	err = s.h.profileRepo.StoreProfile(ctx, pr)
	if err != nil {
		err := fmt.Errorf("failed to store profie: %w", err)
		s.h.logger.WithTrace(ctx).Error("failed to post profile", log.Error("error", err))
		return oapi.PostProfile500JSONResponse{Message: err.Error()}, nil
	}

	return oapi.PostProfile201JSONResponse{
		Id:       pr.ID,
		TenantId: pr.TenantID,
		Email:    pr.Email,
		Name:     pr.Name,
		Nin:      pr.NIN,
		Phone:    pr.Phone,
		Dob:      pr.DOB,
	}, nil
}
