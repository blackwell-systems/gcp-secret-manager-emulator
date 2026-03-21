package authz

import "testing"

func TestGetPermission(t *testing.T) {
	tests := []struct {
		name           string
		operation      string
		wantPermission string
		wantTarget     ResourceTarget
		wantOK         bool
	}{
		// Secret operations
		{
			name:           "CreateSecret",
			operation:      "CreateSecret",
			wantPermission: "secretmanager.secrets.create",
			wantTarget:     ResourceTargetParent,
			wantOK:         true,
		},
		{
			name:           "GetSecret",
			operation:      "GetSecret",
			wantPermission: "secretmanager.secrets.get",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "UpdateSecret",
			operation:      "UpdateSecret",
			wantPermission: "secretmanager.secrets.update",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "DeleteSecret",
			operation:      "DeleteSecret",
			wantPermission: "secretmanager.secrets.delete",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "ListSecrets",
			operation:      "ListSecrets",
			wantPermission: "secretmanager.secrets.list",
			wantTarget:     ResourceTargetParent,
			wantOK:         true,
		},

		// Secret version operations
		{
			name:           "AddSecretVersion",
			operation:      "AddSecretVersion",
			wantPermission: "secretmanager.versions.add",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "AccessSecretVersion",
			operation:      "AccessSecretVersion",
			wantPermission: "secretmanager.versions.access",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "GetSecretVersion",
			operation:      "GetSecretVersion",
			wantPermission: "secretmanager.versions.get",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "ListSecretVersions",
			operation:      "ListSecretVersions",
			wantPermission: "secretmanager.versions.list",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "EnableSecretVersion",
			operation:      "EnableSecretVersion",
			wantPermission: "secretmanager.versions.enable",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "DisableSecretVersion",
			operation:      "DisableSecretVersion",
			wantPermission: "secretmanager.versions.disable",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},
		{
			name:           "DestroySecretVersion",
			operation:      "DestroySecretVersion",
			wantPermission: "secretmanager.versions.destroy",
			wantTarget:     ResourceTargetSelf,
			wantOK:         true,
		},

		// Unknown operations
		{
			name:      "UnknownOperation",
			operation: "UnknownOperation",
			wantOK:    false,
		},
		{
			name:      "EmptyOperation",
			operation: "",
			wantOK:    false,
		},
		{
			name:      "CaseSensitive_lowercase",
			operation: "createsecret",
			wantOK:    false,
		},
		{
			name:      "CaseSensitive_uppercase",
			operation: "CREATESECRET",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm, ok := GetPermission(tt.operation)

			if ok != tt.wantOK {
				t.Errorf("GetPermission(%q) ok = %v, want %v", tt.operation, ok, tt.wantOK)
				return
			}

			if !tt.wantOK {
				return
			}

			if perm.Permission != tt.wantPermission {
				t.Errorf("GetPermission(%q).Permission = %q, want %q", tt.operation, perm.Permission, tt.wantPermission)
			}

			if perm.Target != tt.wantTarget {
				t.Errorf("GetPermission(%q).Target = %v, want %v", tt.operation, perm.Target, tt.wantTarget)
			}
		})
	}
}

func TestOperationPermissions_Completeness(t *testing.T) {
	// Verify all expected operations are mapped
	expectedOps := []string{
		"CreateSecret", "GetSecret", "UpdateSecret", "DeleteSecret", "ListSecrets",
		"AddSecretVersion", "AccessSecretVersion", "GetSecretVersion",
		"ListSecretVersions", "EnableSecretVersion", "DisableSecretVersion",
		"DestroySecretVersion",
	}

	for _, op := range expectedOps {
		if _, ok := OperationPermissions[op]; !ok {
			t.Errorf("OperationPermissions missing operation %q", op)
		}
	}

	// Verify total count matches expected
	if len(OperationPermissions) != len(expectedOps) {
		t.Errorf("OperationPermissions has %d entries, want %d", len(OperationPermissions), len(expectedOps))
	}
}

func TestResourceTarget_Constants(t *testing.T) {
	// Verify the iota values are distinct
	if ResourceTargetSelf == ResourceTargetParent {
		t.Error("ResourceTargetSelf and ResourceTargetParent should have different values")
	}

	// Verify ResourceTargetSelf is 0 (iota start)
	if ResourceTargetSelf != 0 {
		t.Errorf("ResourceTargetSelf = %d, want 0", ResourceTargetSelf)
	}

	// Verify ResourceTargetParent is 1
	if ResourceTargetParent != 1 {
		t.Errorf("ResourceTargetParent = %d, want 1", ResourceTargetParent)
	}
}
