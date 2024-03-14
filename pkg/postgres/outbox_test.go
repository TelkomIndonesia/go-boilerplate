package postgres

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
)

func TestMarshallOutbox(t *testing.T) {
	pr := &profile.Profile{
		ID:       requireUUIDV7(t),
		TenantID: requireUUIDV7(t),
		NIN:      t.Name(),
	}
	ob, err := newOutbox(requireUUIDV7(t), "test", "profile", pr)
	require.NoError(t, err, "should create outbox")

	pr1 := profile.Profile{}
	err = ob.UnmarshalContent(&pr1)
	require.NoError(t, err, "should unmarshall content")
	assert.Equal(t, pr1.ID, pr.ID)
	assert.Equal(t, pr1.TenantID, pr.TenantID)
	assert.Equal(t, pr1.NIN, pr.NIN)

	v, err := json.Marshal(ob)
	require.NoError(t, err, "should be marshallable to json")
	t.Log(string(v))

	m := struct {
		Content *profile.Profile `json:"content"`
	}{}
	err = json.Unmarshal(v, &m)
	require.NoError(t, err, "should unmarshal content to profile")
	assert.Equal(t, m.Content.ID, pr.ID)
	assert.Equal(t, m.Content.TenantID, pr.TenantID)
	assert.Equal(t, m.Content.NIN, pr.NIN)
}
