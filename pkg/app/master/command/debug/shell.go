package debug

import (
	"strings"
	//log "github.com/sirupsen/logrus"
)

const (
	sidKey   = "_SESSION_ID_"
	shellKey = "_SHELL_NAME_"
)

const (
	defaultShellName = "sh"
	bashShellName    = "bash"
)

// NOTES: Mitigating variable expansion done by kubernetes & shell/heredoc
var shellConfig = `
set -eu

cat << 'EOF' > /.mint_debugger_shell_config.sh
#!/bin/sh

if [ -d "/proc/$$$/root/usr/local/sbin/" ]; then
ln -s /proc/$$$/root/usr/local/sbin/ /proc/1/root/.mint_debugger_ulsbin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_ulsbin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/usr/local/bin/" ]; then
ln -s /proc/$$$/root/usr/local/bin/ /proc/1/root/.mint_debugger_ulbin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_ulbin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/usr/sbin/" ]; then
ln -s /proc/$$$/root/usr/sbin/ /proc/1/root/.mint_debugger_usbin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_usbin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/usr/bin/" ]; then
ln -s /proc/$$$/root/usr/bin/ /proc/1/root/.mint_debugger_ubin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_ubin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/sbin/" ]; then
ln -s /proc/$$$/root/sbin/ /proc/1/root/.mint_debugger_sbin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_sbin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/bin/" ]; then
ln -s /proc/$$$/root/bin/ /proc/1/root/.mint_debugger_bin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_bin__SESSION_ID_
fi

chroot /proc/1/root sh
EOF

sh /.mint_debugger_shell_config.sh
`

//exec sh...

func configShell(sessionID string, isK8s bool) string {
	result := strings.ReplaceAll(shellConfig, sidKey, sessionID)
	if isK8s {
		return result
	}

	return strings.ReplaceAll(result, "/$$$/", "/$$/")
}

var shellConfigAlt = `
set -eu

cat << 'EOF' > /.mint_debugger_shell_config.sh
#!/bin/sh

if [ -d "/proc/$$$/root/usr/local/sbin/" ]; then
ln -s /proc/$$$/root/usr/local/sbin/ /proc/1/root/.mint_debugger_ulsbin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_ulsbin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/usr/local/bin/" ]; then
ln -s /proc/$$$/root/usr/local/bin/ /proc/1/root/.mint_debugger_ulbin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_ulbin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/usr/sbin/" ]; then
ln -s /proc/$$$/root/usr/sbin/ /proc/1/root/.mint_debugger_usbin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_usbin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/usr/bin/" ]; then
ln -s /proc/$$$/root/usr/bin/ /proc/1/root/.mint_debugger_ubin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_ubin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/sbin/" ]; then
ln -s /proc/$$$/root/sbin/ /proc/1/root/.mint_debugger_sbin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_sbin__SESSION_ID_
fi

if [ -d "/proc/$$$/root/bin/" ]; then
ln -s /proc/$$$/root/bin/ /proc/1/root/.mint_debugger_bin__SESSION_ID_

export PATH=$PATH:/.mint_debugger_bin__SESSION_ID_
fi

if [ -f "/proc/$$$/root/bin/busybox" ] && [ ! -f "/proc/1/root/bin/busybox" ]; then
ln -s /proc/$$$/root/bin/busybox /proc/1/root/bin/busybox
fi

ln -s /proc/$$$/root/lib/ /proc/1/root/.mint_debugger_lib__SESSION_ID_
ln -s /proc/$$$/root/usr/lib/ /proc/1/root/.mint_debugger_ulib__SESSION_ID_

ln -s /proc/$$$/root/usr/lib/libncursesw.so.6 /proc/1/root/usr/lib/libncursesw.so.6
ln -s /proc/$$$/root/usr/lib/libncursesw.so.6.4 /proc/1/root/usr/lib/libncursesw.so.6.4

#if [ -n "$LD_LIBRARY_PATH" ]; then
#export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/.mint_debugger_ulib__SESSION_ID_
#  export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/.mint_debugger_lib__SESSION_ID_:/.mint_debugger_ulib__SESSION_ID_
#else
#export LD_LIBRARY_PATH=/.mint_debugger_ulib__SESSION_ID_
#  export LD_LIBRARY_PATH=/.mint_debugger_lib__SESSION_ID_:/.mint_debugger_ulib__SESSION_ID_
#fi

chroot /proc/1/root bash
EOF

sh /.mint_debugger_shell_config.sh
`

func configShellAlt(sessionID string, isK8s bool) string {
	result := strings.ReplaceAll(shellConfigAlt, sidKey, sessionID)
	if isK8s {
		return result
	}

	return strings.ReplaceAll(result, "/$$$/", "/$$/")
}
