package application

import (
	"slices"

	"go.klarlabs.de/vitra/domain"
)

// CapabilityGateway evaluates capability grants for IPC callers.
// Denials are deterministic: the first matching grant that allows wins;
// otherwise the most specific denial from scanned grants is returned, or
// DenialNoGrant when no grant mentions the permission at all.
type CapabilityGateway struct {
	grants []*domain.CapabilityGrant
}

// NewCapabilityGateway constructs a gateway over the given grants.
func NewCapabilityGateway(grants ...*domain.CapabilityGrant) *CapabilityGateway {
	copied := make([]*domain.CapabilityGrant, 0, len(grants))
	for _, g := range grants {
		if g != nil {
			copied = append(copied, g)
		}
	}
	return &CapabilityGateway{grants: copied}
}

// Grants returns the grants known to the gateway.
func (gw *CapabilityGateway) Grants() []*domain.CapabilityGrant {
	return append([]*domain.CapabilityGrant(nil), gw.grants...)
}

// Authorize checks whether caller may exercise permission for resourcePath.
func (gw *CapabilityGateway) Authorize(caller domain.Caller, permission domain.PermissionName, resourcePath string) domain.Decision {
	if permission == "" {
		return domain.Decision{
			Code:   domain.DenialPermissionAbsent,
			Reason: "permission is required",
		}
	}
	var bestDenial domain.Decision
	foundMention := false
	for _, g := range gw.grants {
		d := g.Authorize(caller, permission, resourcePath)
		if d.Allowed {
			return d
		}
		// Track denials only from grants that at least list this permission,
		// so window/origin mismatches surface over a generic no_grant.
		if d.Code == domain.DenialPermissionAbsent {
			continue
		}
		foundMention = true
		if bestDenial.Code == "" || denialSpecificity(d.Code) > denialSpecificity(bestDenial.Code) {
			bestDenial = d
		}
	}
	if !foundMention {
		return domain.Decision{
			Permission: permission,
			Code:       domain.DenialNoGrant,
			Reason:     "no capability grant authorizes this permission",
		}
	}
	bestDenial.Permission = permission
	return bestDenial
}

func denialSpecificity(code domain.DenialCode) int {
	switch code {
	case domain.DenialPathDenied:
		return 40
	case domain.DenialPathOutOfScope:
		return 30
	case domain.DenialOriginMismatch:
		return 20
	case domain.DenialWindowMismatch:
		return 10
	default:
		return 0
	}
}

// Inspect returns the effective privileged surface for window+origin.
func (gw *CapabilityGateway) Inspect(window domain.WindowID, origin domain.Origin) domain.EffectiveSurface {
	surface := domain.EffectiveSurface{
		Window: window,
		Origin: origin,
	}
	seenGrant := map[domain.GrantName]struct{}{}
	for _, g := range gw.grants {
		if !slices.Contains(g.Windows(), window) || !slices.Contains(g.Origins(), origin) {
			continue
		}
		if _, ok := seenGrant[g.Name()]; !ok {
			seenGrant[g.Name()] = struct{}{}
			surface.GrantNames = append(surface.GrantNames, g.Name())
		}
		for _, p := range g.Permissions() {
			ep := domain.EffectivePermission{Name: p.Name, Grant: g.Name()}
			if p.PathScope != nil {
				ep.PathAllow = append([]string(nil), p.PathScope.Allow...)
				ep.PathDeny = append([]string(nil), p.PathScope.Deny...)
			}
			surface.Permissions = append(surface.Permissions, ep)
		}
	}
	return surface
}
