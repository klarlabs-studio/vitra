package domain

// EffectiveSurface projects the inspectable privileged surface for a window
// at a given origin — what vitra inspect should show.
type EffectiveSurface struct {
	Window      WindowID
	Origin      Origin
	GrantNames  []GrantName
	Permissions []EffectivePermission
}

// EffectivePermission is one inspectable permission entry.
type EffectivePermission struct {
	Name      PermissionName
	Grant     GrantName
	PathAllow []string
	PathDeny  []string
}
