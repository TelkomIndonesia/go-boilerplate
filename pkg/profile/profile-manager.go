package profile

import (
	"context"
	"fmt"
	"time"
)

type ProfileManager struct {
	PR ProfileRepository
	TR TenantRepository
}

func (pm ProfileManager) ValidateProfile(ctx context.Context, p *Profile) (err error) {
	t, err := pm.TR.FetchTenant(ctx, p.TenantID)
	if err != nil {
		return fmt.Errorf("failed to fetch tenant: %w", err)
	}
	if t == nil {
		return fmt.Errorf("tenant not found: %w", err)
	}
	if t.Expire.After(time.Now()) {
		return fmt.Errorf("tenant expired")
	}
	return
}
