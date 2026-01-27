package authz

import "strings"

// NormalizeSecretResource extracts the secret path from a full resource name.
// Input: projects/{p}/secrets/{s}/versions/{v}
// Output: projects/{p}/secrets/{s}
func NormalizeSecretResource(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) >= 4 && parts[0] == "projects" && parts[2] == "secrets" {
		return strings.Join(parts[:4], "/")
	}
	return name
}

// NormalizeSecretVersionResource preserves the full version path.
// Input: projects/{p}/secrets/{s}/versions/{v}
// Output: projects/{p}/secrets/{s}/versions/{v}
func NormalizeSecretVersionResource(name string) string {
	// Keep full path for version-specific permissions
	return name
}

// NormalizeParentForCreate returns the parent resource for create operations.
// Input: projects/{p}
// Output: projects/{p}
func NormalizeParentForCreate(parent string) string {
	return parent
}
