package buildflags

import (
	"github.com/moby/buildkit/util/entitlements"
	"github.com/pkg/errors"
)

func ParseEntitlements(in []string) ([]entitlements.Entitlement, error) {
	out := make([]entitlements.Entitlement, 0, len(in))
	for _, v := range in {
		switch v {
		case "security.insecure":
			out = append(out, entitlements.EntitlementSecurityInsecure)
		case "network.host":
			out = append(out, entitlements.EntitlementNetworkHost)
		default:
			return nil, errors.Errorf("invalid entitlement: %v", v)
		}
	}
	return out, nil
}
