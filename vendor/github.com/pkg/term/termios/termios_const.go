// +build !windows,!solaris

package termios

const (
	TCIFLUSH  = 0
	TCOFLUSH  = 1
	TCIOFLUSH = 2

	TCSANOW   = 0
	TCSADRAIN = 1
	TCSAFLUSH = 2
)
