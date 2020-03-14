package termios

const (
	_IOC_PARAM_SHIFT = 13
	_IOC_PARAM_MASK  = (1 << _IOC_PARAM_SHIFT) - 1
)

func _IOC_PARM_LEN(ioctl uintptr) uintptr {
	return (ioctl >> 16) & _IOC_PARAM_MASK
}
