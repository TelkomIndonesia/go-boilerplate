package httpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver/internal/oapi"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

// GetProfile implements oapi.StrictServerInterface.
func (s oapiServerImplementation) GetProfile(ctx context.Context, request oapi.GetProfileRequestObject) (oapi.GetProfileResponseObject, error) {
	ctx = context.WithValue(ctx, "test", "test")
	pr, err := s.h.profileRepo.FetchProfile(ctx, request.TenantId, request.ProfileId)
	if err != nil {
		return nil, err
	}
	if pr == nil {
		return oapi.GetProfile404Response{}, nil
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
		return nil, fmt.Errorf("fail to create id: %w", err)
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
			s.h.logger.Error("fail to validate repo", log.Error("error", err))
			return nil, err
		}
	}

	err = s.h.profileRepo.StoreProfile(ctx, pr)
	if err != nil {
		return nil, err
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
