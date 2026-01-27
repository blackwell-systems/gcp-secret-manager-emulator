package authz

// ResourceTarget defines whether permission check is against parent or the resource itself
type ResourceTarget int

const (
	ResourceTargetSelf   ResourceTarget = iota // Check against resource itself
	ResourceTargetParent                       // Check against parent (for creates)
)

// PermissionCheck defines the permission and resource target for an operation
type PermissionCheck struct {
	Permission string
	Target     ResourceTarget
}

// OperationPermissions maps Secret Manager operations to their required permissions
var OperationPermissions = map[string]PermissionCheck{
	// Secret operations
	"CreateSecret": {
		Permission: "secretmanager.secrets.create",
		Target:     ResourceTargetParent,
	},
	"GetSecret": {
		Permission: "secretmanager.secrets.get",
		Target:     ResourceTargetSelf,
	},
	"UpdateSecret": {
		Permission: "secretmanager.secrets.update",
		Target:     ResourceTargetSelf,
	},
	"DeleteSecret": {
		Permission: "secretmanager.secrets.delete",
		Target:     ResourceTargetSelf,
	},
	"ListSecrets": {
		Permission: "secretmanager.secrets.list",
		Target:     ResourceTargetParent,
	},

	// Secret version operations
	"AddSecretVersion": {
		Permission: "secretmanager.versions.add",
		Target:     ResourceTargetSelf, // Check against secret
	},
	"AccessSecretVersion": {
		Permission: "secretmanager.versions.access",
		Target:     ResourceTargetSelf, // Check against version or secret
	},
	"GetSecretVersion": {
		Permission: "secretmanager.versions.get",
		Target:     ResourceTargetSelf, // Check against version or secret
	},
	"ListSecretVersions": {
		Permission: "secretmanager.versions.list",
		Target:     ResourceTargetSelf, // Check against secret
	},
	"EnableSecretVersion": {
		Permission: "secretmanager.versions.enable",
		Target:     ResourceTargetSelf, // Check against version or secret
	},
	"DisableSecretVersion": {
		Permission: "secretmanager.versions.disable",
		Target:     ResourceTargetSelf, // Check against version or secret
	},
	"DestroySecretVersion": {
		Permission: "secretmanager.versions.destroy",
		Target:     ResourceTargetSelf, // Check against version or secret
	},
}

// GetPermission returns the permission check for an operation
func GetPermission(operation string) (PermissionCheck, bool) {
	perm, ok := OperationPermissions[operation]
	return perm, ok
}
