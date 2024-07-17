package httpserver

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver/internal/oapi"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	profilemock "github.com/telkomindonesia/go-boilerplate/pkg/profile/mock"
)

func TestGetProfile(t *testing.T) {
	// init mock
	pr, tr := profilemock.NewMockProfileRepository(t), profilemock.NewMockTenantRepository(t)

	// init server
	h, err := New(
		WithProfileRepository(pr),
		WithTenantRepository(tr),
	)
	require.NoError(t, err)
	require.NotNil(t, h)
	s := oapiServerImplementation{h: h}

	// set up test data
	ctx := context.Background()
	tid := uuid.New()
	pid := uuid.New()

	// set up expectation
	pr.EXPECT().FetchProfile(mock.Anything, tid, pid).Return(&profile.Profile{
		TenantID: tid,
		ID:       pid,
		NIN:      "1",
	}, nil)

	// exec
	res, err := s.GetProfile(ctx, oapi.GetProfileRequestObject{
		TenantId:  tid,
		ProfileId: pid,
	})

	// verify
	require.NoError(t, err)
	require.IsType(t, oapi.GetProfile200JSONResponse{}, res)
	res200 := res.(oapi.GetProfile200JSONResponse)
	assert.Equal(t, oapi.GetProfile200JSONResponse{
		TenantId: tid,
		Id:       pid,
		Nin:      "1",
	}, res200)
}
