package sysenv

func HasDSImageFlag() bool {
	return false
}

func InDSContainer() (bool, bool) {
	return false, false
}
