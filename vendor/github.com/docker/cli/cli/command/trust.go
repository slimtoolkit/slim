package command

import (
	"github.com/spf13/pflag"
)

// AddTrustVerificationFlags adds content trust flags to the provided flagset
func AddTrustVerificationFlags(fs *pflag.FlagSet, v *bool, trusted bool) {
	fs.BoolVar(v, "disable-content-trust", !trusted, "Skip image verification")
}

// AddTrustSigningFlags adds "signing" flags to the provided flagset
func AddTrustSigningFlags(fs *pflag.FlagSet, v *bool, trusted bool) {
	fs.BoolVar(v, "disable-content-trust", !trusted, "Skip image signing")
}
