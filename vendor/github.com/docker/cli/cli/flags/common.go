package flags

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/opts"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const (
	// DefaultCaFile is the default filename for the CA pem file
	DefaultCaFile = "ca.pem"
	// DefaultKeyFile is the default filename for the key pem file
	DefaultKeyFile = "key.pem"
	// DefaultCertFile is the default filename for the cert pem file
	DefaultCertFile = "cert.pem"
	// FlagTLSVerify is the flag name for the TLS verification option
	FlagTLSVerify = "tlsverify"
	// FormatHelp describes the --format flag behavior for list commands
	FormatHelp = `Format output using a custom template:
'table':            Print output in table format with column headers (default)
'table TEMPLATE':   Print output in table format using the given Go template
'json':             Print in JSON format
'TEMPLATE':         Print output using the given Go template.
Refer to https://docs.docker.com/go/formatting/ for more information about formatting output with templates`
	// InspectFormatHelp describes the --format flag behavior for inspect commands
	InspectFormatHelp = `Format output using a custom template:
'json':             Print in JSON format
'TEMPLATE':         Print output using the given Go template.
Refer to https://docs.docker.com/go/formatting/ for more information about formatting output with templates`
)

var (
	dockerCertPath  = os.Getenv(client.EnvOverrideCertPath)
	dockerTLSVerify = os.Getenv(client.EnvTLSVerify) != ""
	// TODO(thaJeztah) the 'DOCKER_TLS' environment variable is not documented, and does not have a const.
	dockerTLS = os.Getenv("DOCKER_TLS") != ""
)

// CommonOptions are options common to both the client and the daemon.
type CommonOptions struct {
	Debug      bool
	Hosts      []string
	LogLevel   string
	TLS        bool
	TLSVerify  bool
	TLSOptions *tlsconfig.Options
	Context    string
}

// NewCommonOptions returns a new CommonOptions
func NewCommonOptions() *CommonOptions {
	return &CommonOptions{}
}

// InstallFlags adds flags for the common options on the FlagSet
func (commonOpts *CommonOptions) InstallFlags(flags *pflag.FlagSet) {
	if dockerCertPath == "" {
		dockerCertPath = config.Dir()
	}

	flags.BoolVarP(&commonOpts.Debug, "debug", "D", false, "Enable debug mode")
	flags.StringVarP(&commonOpts.LogLevel, "log-level", "l", "info", `Set the logging level ("debug"|"info"|"warn"|"error"|"fatal")`)
	flags.BoolVar(&commonOpts.TLS, "tls", dockerTLS, "Use TLS; implied by --tlsverify")
	flags.BoolVar(&commonOpts.TLSVerify, FlagTLSVerify, dockerTLSVerify, "Use TLS and verify the remote")

	// TODO use flag flags.String("identity"}, "i", "", "Path to libtrust key file")

	commonOpts.TLSOptions = &tlsconfig.Options{
		CAFile:   filepath.Join(dockerCertPath, DefaultCaFile),
		CertFile: filepath.Join(dockerCertPath, DefaultCertFile),
		KeyFile:  filepath.Join(dockerCertPath, DefaultKeyFile),
	}
	tlsOptions := commonOpts.TLSOptions
	flags.Var(opts.NewQuotedString(&tlsOptions.CAFile), "tlscacert", "Trust certs signed only by this CA")
	flags.Var(opts.NewQuotedString(&tlsOptions.CertFile), "tlscert", "Path to TLS certificate file")
	flags.Var(opts.NewQuotedString(&tlsOptions.KeyFile), "tlskey", "Path to TLS key file")

	// opts.ValidateHost is not used here, so as to allow connection helpers
	hostOpt := opts.NewNamedListOptsRef("hosts", &commonOpts.Hosts, nil)
	flags.VarP(hostOpt, "host", "H", "Daemon socket(s) to connect to")
	flags.StringVarP(&commonOpts.Context, "context", "c", "",
		`Name of the context to use to connect to the daemon (overrides `+client.EnvOverrideHost+` env var and default context set with "docker context use")`)
}

// SetDefaultOptions sets default values for options after flag parsing is
// complete
func (commonOpts *CommonOptions) SetDefaultOptions(flags *pflag.FlagSet) {
	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on TLS
	// TLSVerify can be true even if not set due to DOCKER_TLS_VERIFY env var, so we need
	// to check that here as well
	if flags.Changed(FlagTLSVerify) || commonOpts.TLSVerify {
		commonOpts.TLS = true
	}

	if !commonOpts.TLS {
		commonOpts.TLSOptions = nil
	} else {
		tlsOptions := commonOpts.TLSOptions
		tlsOptions.InsecureSkipVerify = !commonOpts.TLSVerify

		// Reset CertFile and KeyFile to empty string if the user did not specify
		// the respective flags and the respective default files were not found.
		if !flags.Changed("tlscert") {
			if _, err := os.Stat(tlsOptions.CertFile); os.IsNotExist(err) {
				tlsOptions.CertFile = ""
			}
		}
		if !flags.Changed("tlskey") {
			if _, err := os.Stat(tlsOptions.KeyFile); os.IsNotExist(err) {
				tlsOptions.KeyFile = ""
			}
		}
	}
}

// SetLogLevel sets the logrus logging level
func SetLogLevel(logLevel string) {
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to parse logging level: %s\n", logLevel)
			os.Exit(1)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
