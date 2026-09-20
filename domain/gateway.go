package domain

// CapabilityGateway evaluates capability grants for IPC callers.
// Denials are deterministic: the first matching grant that allows wins;
// otherwise the most specific denial from scanned grants is returned, or
// DenialNoGrant when no grant mentions the permission at all.
type CapabilityGateway struct {
	grants []*CapabilityGrant
}

// NewCapabilityGateway constructs a gateway over the given grants.
func NewCapabilityGateway(grants ...*CapabilityGrant) *CapabilityGateway {
	copied := make([]*CapabilityGrant, 0, len(grants))
	for _, g := range grants {
		if g != nil {
			copied = append(copied, g)
		}
	}
	return &CapabilityGateway{grants: copied}
}

// Grants returns the grants known to the gateway.
func (gw *CapabilityGateway) Grants() []*CapabilityGrant {
	return append([]*CapabilityGrant(nil), gw.grants...)
}

// Authorize checks whether caller may exercise permission for resourcePath.
func (gw *CapabilityGateway) Authorize(caller Caller, permission PermissionName, resourcePath string) Decision {
	if permission == "" {
		return Decision{
			Code:   DenialPermissionAbsent,
			Reason: "permission is required",
		}
	}
	var bestDenial Decision
	foundMention := false
	for _, g := range gw.grants {
		d := g.Authorize(caller, permission, resourcePath)
		if d.Allowed {
			return d
		}
		// Track denials only from grants that at least list this permission,
		// so window/origin mismatches surface over a generic no_grant.
		if d.Code == DenialPermissionAbsent {
			continue
		}
		foundMention = true
		if bestDenial.Code == "" || denialSpecificity(d.Code) > denialSpecificity(bestDenial.Code) {
			bestDenial = d
		}
	}
	if !foundMention {
		return Decision{
			Permission: permission,
			Code:       DenialNoGrant,
			Reason:     "no capability grant authorizes this permission",
		}
	}
	bestDenial.Permission = permission
	return bestDenial
}

func denialSpecificity(code DenialCode) int {
	switch code {
	case DenialPathDenied:
		return 40
	case DenialPathOutOfScope:
		return 30
	case DenialOriginMismatch:
		return 20
	case DenialWindowMismatch:
		return 10
	default:
		return 0
	}
}

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

// Inspect returns the effective privileged surface for window+origin.
func (gw *CapabilityGateway) Inspect(window WindowID, origin Origin) EffectiveSurface {
	surface := EffectiveSurface{
		Window: window,
		Origin: origin,
	}
	seenGrant := map[GrantName]struct{}{}
	for _, g := range gw.grants {
		if !containsWindow(g.windows, window) || !containsOrigin(g.origins, origin) {
			continue
		}
		if _, ok := seenGrant[g.name]; !ok {
			seenGrant[g.name] = struct{}{}
			surface.GrantNames = append(surface.GrantNames, g.name)
		}
		for _, p := range g.permissions {
			ep := EffectivePermission{Name: p.Name, Grant: g.name}
			if p.PathScope != nil {
				ep.PathAllow = append([]string(nil), p.PathScope.Allow...)
				ep.PathDeny = append([]string(nil), p.PathScope.Deny...)
			}
			surface.Permissions = append(surface.Permissions, ep)
		}
	}
	return surface
}
