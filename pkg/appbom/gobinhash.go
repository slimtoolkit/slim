//go:build !appbom_noembed
// +build !appbom_noembed

package appbom

import (
	_ "embed"
)

//go:embed gobinhash
var goBinHash string
