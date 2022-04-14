package command

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	configtypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/cli/cli/debug"
	"github.com/docker/cli/cli/streams"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/registry"
	"github.com/moby/term"
	"github.com/pkg/errors"
)

// ElectAuthServer returns the default registry to use (by asking the daemon)
func ElectAuthServer(ctx context.Context, cli Cli) string {
	// The daemon `/info` endpoint informs us of the default registry being
	// used. This is essential in cross-platforms environment, where for
	// example a Linux client might be interacting with a Windows daemon, hence
	// the default registry URL might be Windows specific.
	info, err := cli.Client().Info(ctx)
	if err != nil {
		// Daemon is not responding so use system default.
		if debug.IsEnabled() {
			// Only report the warning if we're in debug mode to prevent nagging during engine initialization workflows
			fmt.Fprintf(cli.Err(), "Warning: failed to get default registry endpoint from daemon (%v). Using system default: %s\n", err, registry.IndexServer)
		}
		return registry.IndexServer
	}
	if info.IndexServerAddress == "" {
		if debug.IsEnabled() {
			fmt.Fprintf(cli.Err(), "Warning: Empty registry endpoint from daemon. Using system default: %s\n", registry.IndexServer)
		}
		return registry.IndexServer
	}
	return info.IndexServerAddress
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func EncodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// RegistryAuthenticationPrivilegedFunc returns a RequestPrivilegeFunc from the specified registry index info
// for the given command.
func RegistryAuthenticationPrivilegedFunc(cli Cli, index *registrytypes.IndexInfo, cmdName string) types.RequestPrivilegeFunc {
	return func() (string, error) {
		fmt.Fprintf(cli.Out(), "\nPlease login prior to %s:\n", cmdName)
		indexServer := registry.GetAuthConfigKey(index)
		isDefaultRegistry := indexServer == ElectAuthServer(context.Background(), cli)
		authConfig, err := GetDefaultAuthConfig(cli, true, indexServer, isDefaultRegistry)
		if err != nil {
			fmt.Fprintf(cli.Err(), "Unable to retrieve stored credentials for %s, error: %s.\n", indexServer, err)
		}
		err = ConfigureAuth(cli, "", "", &authConfig, isDefaultRegistry)
		if err != nil {
			return "", err
		}
		return EncodeAuthToBase64(authConfig)
	}
}

// ResolveAuthConfig is like registry.ResolveAuthConfig, but if using the
// default index, it uses the default index name for the daemon's platform,
// not the client's platform.
func ResolveAuthConfig(ctx context.Context, cli Cli, index *registrytypes.IndexInfo) types.AuthConfig {
	configKey := index.Name
	if index.Official {
		configKey = ElectAuthServer(ctx, cli)
	}

	a, _ := cli.ConfigFile().GetAuthConfig(configKey)
	return types.AuthConfig(a)
}

// GetDefaultAuthConfig gets the default auth config given a serverAddress
// If credentials for given serverAddress exists in the credential store, the configuration will be populated with values in it
func GetDefaultAuthConfig(cli Cli, checkCredStore bool, serverAddress string, isDefaultRegistry bool) (types.AuthConfig, error) {
	if !isDefaultRegistry {
		serverAddress = registry.ConvertToHostname(serverAddress)
	}
	var authconfig = configtypes.AuthConfig{}
	var err error
	if checkCredStore {
		authconfig, err = cli.ConfigFile().GetAuthConfig(serverAddress)
		if err != nil {
			return types.AuthConfig{
				ServerAddress: serverAddress,
			}, err
		}
	}
	authconfig.ServerAddress = serverAddress
	authconfig.IdentityToken = ""
	res := types.AuthConfig(authconfig)
	return res, nil
}

// ConfigureAuth handles prompting of user's username and password if needed
func ConfigureAuth(cli Cli, flUser, flPassword string, authconfig *types.AuthConfig, isDefaultRegistry bool) error {
	// On Windows, force the use of the regular OS stdin stream. Fixes #14336/#14210
	if runtime.GOOS == "windows" {
		cli.SetIn(streams.NewIn(os.Stdin))
	}

	// Some links documenting this:
	// - https://code.google.com/archive/p/mintty/issues/56
	// - https://github.com/docker/docker/issues/15272
	// - https://mintty.github.io/ (compatibility)
	// Linux will hit this if you attempt `cat | docker login`, and Windows
	// will hit this if you attempt docker login from mintty where stdin
	// is a pipe, not a character based console.
	if flPassword == "" && !cli.In().IsTerminal() {
		return errors.Errorf("Error: Cannot perform an interactive login from a non TTY device")
	}

	authconfig.Username = strings.TrimSpace(authconfig.Username)

	if flUser = strings.TrimSpace(flUser); flUser == "" {
		if isDefaultRegistry {
			// if this is a default registry (docker hub), then display the following message.
			fmt.Fprintln(cli.Out(), "Login with your Docker ID to push and pull images from Docker Hub. If you don't have a Docker ID, head over to https://hub.docker.com to create one.")
		}
		promptWithDefault(cli.Out(), "Username", authconfig.Username)
		flUser = readInput(cli.In(), cli.Out())
		flUser = strings.TrimSpace(flUser)
		if flUser == "" {
			flUser = authconfig.Username
		}
	}
	if flUser == "" {
		return errors.Errorf("Error: Non-null Username Required")
	}
	if flPassword == "" {
		oldState, err := term.SaveState(cli.In().FD())
		if err != nil {
			return err
		}
		fmt.Fprintf(cli.Out(), "Password: ")
		term.DisableEcho(cli.In().FD(), oldState)

		flPassword = readInput(cli.In(), cli.Out())
		fmt.Fprint(cli.Out(), "\n")

		term.RestoreTerminal(cli.In().FD(), oldState)
		if flPassword == "" {
			return errors.Errorf("Error: Password Required")
		}
	}

	authconfig.Username = flUser
	authconfig.Password = flPassword

	return nil
}

func readInput(in io.Reader, out io.Writer) string {
	reader := bufio.NewReader(in)
	line, _, err := reader.ReadLine()
	if err != nil {
		fmt.Fprintln(out, err.Error())
		os.Exit(1)
	}
	return string(line)
}

func promptWithDefault(out io.Writer, prompt string, configDefault string) {
	if configDefault == "" {
		fmt.Fprintf(out, "%s: ", prompt)
	} else {
		fmt.Fprintf(out, "%s (%s): ", prompt, configDefault)
	}
}

// RetrieveAuthTokenFromImage retrieves an encoded auth token given a complete image
func RetrieveAuthTokenFromImage(ctx context.Context, cli Cli, image string) (string, error) {
	// Retrieve encoded auth token from the image reference
	authConfig, err := resolveAuthConfigFromImage(ctx, cli, image)
	if err != nil {
		return "", err
	}
	encodedAuth, err := EncodeAuthToBase64(authConfig)
	if err != nil {
		return "", err
	}
	return encodedAuth, nil
}

// resolveAuthConfigFromImage retrieves that AuthConfig using the image string
func resolveAuthConfigFromImage(ctx context.Context, cli Cli, image string) (types.AuthConfig, error) {
	registryRef, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return types.AuthConfig{}, err
	}
	repoInfo, err := registry.ParseRepositoryInfo(registryRef)
	if err != nil {
		return types.AuthConfig{}, err
	}
	return ResolveAuthConfig(ctx, cli, repoInfo.Index), nil
}
