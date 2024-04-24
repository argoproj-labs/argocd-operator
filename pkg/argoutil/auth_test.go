package argoutil

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestGenerateArgoAdminPassword(t *testing.T) {
	password, err := GenerateArgoAdminPassword()
	assert.NoError(t, err)
	assert.NotNil(t, password)
}

func TestGenerateArgoServerSessionKey(t *testing.T) {
	password, err := GenerateArgoServerSessionKey()
	assert.NoError(t, err)
	assert.NotNil(t, password)
}

func TestHasArgoAdminPasswordChanged(t *testing.T) {
	t.Run("Admin Password Changed", func(t *testing.T) {
		old_admin_password, err := GenerateArgoAdminPassword()
		if err != nil {
			t.Errorf("Error when generating admin password")

		}
		old_password := &corev1.Secret{
			Data: map[string][]byte{
				"admin-password": old_admin_password,
			},
		}

		new_admin_password, err := GenerateArgoAdminPassword()
		if err != nil {
			t.Errorf("Error when generating admin password")

		}
		new_password := &corev1.Secret{
			Data: map[string][]byte{
				"admin-password": new_admin_password,
			},
		}

		got := HasArgoAdminPasswordChanged(old_password, new_password)
		if got != true {
			t.Errorf("HasAdminPasswordChanged() = %v, want true", got)
		}
	})
	t.Run("Admin Password Not Changed", func(t *testing.T) {
		old_admin_password, err := GenerateArgoAdminPassword()
		if err != nil {
			t.Errorf("Error when generating admin password")

		}
		old_password := &corev1.Secret{
			Data: map[string][]byte{
				"admin-password": old_admin_password,
			},
		}

		got := HasArgoAdminPasswordChanged(old_password, old_password)
		if got != false {
			t.Errorf("HasAdminPasswordChanged() = %v, want false", got)
		}
	})
}
