package saas

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestTrialInput(t *testing.T) {
	valid := TrialInput{Name: " Company ", OwnerName: " Owner ", Email: " OWNER@example.com ", Password: "safe-test-password", PlanID: uuid.New()}
	require.NoError(t, valid.Validate())
	require.Equal(t, "owner@example.com", valid.Email)
	for _, password := range []string{"short", strings.Repeat("a", 73), strings.Repeat("á", 37)} {
		input := valid
		input.Password = password
		require.Error(t, input.Validate())
	}
	invalid := valid
	invalid.Email = "Name <owner@example.com>"
	require.Error(t, invalid.Validate())
}
