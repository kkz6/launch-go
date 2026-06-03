package staff

import (
	"testing"

	"github.com/stretchr/testify/assert"

	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

func TestStaffRole_IsValid(t *testing.T) {
	assert.True(t, stafftypes.StaffRoleSupport.IsValid())
	assert.True(t, stafftypes.StaffRoleSuperAdmin.IsValid())
	assert.False(t, stafftypes.StaffRole("").IsValid())
	assert.False(t, stafftypes.StaffRole("owner").IsValid())
}

func TestStaffRole_Level(t *testing.T) {
	assert.Equal(t, 2, stafftypes.StaffRoleSuperAdmin.Level())
	assert.Equal(t, 1, stafftypes.StaffRoleSupport.Level())
	assert.Equal(t, 0, stafftypes.StaffRole("nonsense").Level())
}

func TestStaffRole_Capabilities(t *testing.T) {
	assert.True(t, stafftypes.StaffRoleSupport.CanViewCustomerData())
	assert.True(t, stafftypes.StaffRoleSupport.CanImpersonate())
	assert.True(t, stafftypes.StaffRoleSupport.CanBypassSubscription())
	assert.False(t, stafftypes.StaffRoleSupport.CanManageProductConfig())
	assert.True(t, stafftypes.StaffRoleSuperAdmin.CanManageProductConfig())
}
