package buildflags

import (
	"strings"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	"github.com/moby/buildkit/util/gitutil"
)

func ParseSSHSpecs(sl []string) (session.Attachable, error) {
	configs := make([]sshprovider.AgentConfig, 0, len(sl))
	for _, v := range sl {
		c, err := parseSSH(v)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *c)
	}
	return sshprovider.NewSSHAgentProvider(configs)
}

func parseSSH(value string) (*sshprovider.AgentConfig, error) {
	parts := strings.SplitN(value, "=", 2)
	cfg := sshprovider.AgentConfig{
		ID: parts[0],
	}
	if len(parts) > 1 {
		cfg.Paths = strings.Split(parts[1], ",")
	}
	return &cfg, nil
}

// IsGitSSH returns true if the given repo URL is accessed over ssh
func IsGitSSH(url string) bool {
	_, gitProtocol := gitutil.ParseProtocol(url)
	return gitProtocol == gitutil.SSHProtocol
}
