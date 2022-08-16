package reverse

import (
	"fmt"
	"testing"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T) {
	type TestData struct {
		input                    string
		reconstructedHealthcheck string
		expectedHealthConf       dockerclient.HealthConfig
	}

	testHealthCheckData := []TestData{
		{
			input:                    `HEALTHCHECK &{["CMD" "curl" "--fail-with-body" "127.0.0.1:5000"] "15s" "1s" "20s" '\x0A'}`,
			reconstructedHealthcheck: `HEALTHCHECK --interval=15s --timeout=1s --start-period=20s --retries=10 CMD ["curl", "--fail-with-body", "127.0.0.1:5000"]`,
			expectedHealthConf: dockerclient.HealthConfig{
				Interval:    15 * time.Second,
				Timeout:     1 * time.Second,
				StartPeriod: 20 * time.Second,
				Retries:     10,
				Test:        []string{"CMD", "curl", "--fail-with-body", "127.0.0.1:5000"},
			},
		},
		{
			input:                    `HEALTHCHECK &{["CMD-SHELL" "cat /etc/resolv.conf"] "20s" "10s" "5s" '\x02'}`,
			reconstructedHealthcheck: "HEALTHCHECK --interval=20s --timeout=10s --start-period=5s --retries=2 CMD cat /etc/resolv.conf",
			expectedHealthConf: dockerclient.HealthConfig{
				Interval:    20 * time.Second,
				Timeout:     10 * time.Second,
				StartPeriod: 5 * time.Second,
				Retries:     2,
				Test:        []string{"CMD-SHELL", "cat /etc/resolv.conf"},
			},
		},
		{
			input:                    `HEALTHCHECK &{["CMD-SHELL" "/opt/up-yet.sh"] "0s" "0s" "0s" '\x00'}`,
			reconstructedHealthcheck: "HEALTHCHECK CMD /opt/up-yet.sh",
			expectedHealthConf: dockerclient.HealthConfig{
				Test:        []string{"CMD-SHELL", "/opt/up-yet.sh"},
				Interval:    30 * time.Second,
				Timeout:     30 * time.Second,
				StartPeriod: 0 * time.Second,
				Retries:     3,
			},
		},
		{
			input:                    `HEALTHCHECK &{["CMD" "cat" "/etc/resolv.conf"] "0s" "0s" "0s" '!'}`,
			reconstructedHealthcheck: `HEALTHCHECK --retries=33 CMD ["cat", "/etc/resolv.conf"]`,
			expectedHealthConf: dockerclient.HealthConfig{
				Test:        []string{"CMD", "cat", "/etc/resolv.conf"},
				Interval:    30 * time.Second,
				Timeout:     30 * time.Second,
				StartPeriod: 0 * time.Second,
				Retries:     33,
			},
		},
		{
			input:                    `HEALTHCHECK &{["CMD-SHELL" "ss -tulpn|grep 22"] "2s" "0s" "0s" '\U00051615'}`,
			reconstructedHealthcheck: `HEALTHCHECK --interval=2s --retries=333333 CMD ss -tulpn|grep 22`,
			expectedHealthConf: dockerclient.HealthConfig{
				Test:        []string{"CMD-SHELL", "ss -tulpn|grep 22"},
				Interval:    2 * time.Second,
				Timeout:     30 * time.Second,
				StartPeriod: 0 * time.Second,
				Retries:     333333,
			},
		},
		{
			input:                    `HEALTHCHECK &{["CMD" "/bin/uptime"] "0s" "1h23m20s" "0s" '\n'}`,
			reconstructedHealthcheck: `HEALTHCHECK --timeout=1h23m20s --retries=10 CMD ["/bin/uptime"]`,
			expectedHealthConf: dockerclient.HealthConfig{
				Test:        []string{"CMD", "/bin/uptime"},
				Interval:    30 * time.Second,
				Timeout:     5000 * time.Second,
				StartPeriod: 0 * time.Second,
				Retries:     10,
			},
		},
	}

	for retries := 1; retries <= 31; retries++ {
		testHealthCheckData = append(testHealthCheckData, TestData{
			input:                    fmt.Sprintf(`HEALTHCHECK &{["CMD" "/bin/uptime"] "0s" "1h23m20s" "0s" '%q'}`, retries),
			reconstructedHealthcheck: fmt.Sprintf(`HEALTHCHECK --timeout=1h23m20s --retries=%d CMD ["/bin/uptime"]`, retries),
			expectedHealthConf: dockerclient.HealthConfig{
				Test:        []string{"CMD", "/bin/uptime"},
				Interval:    30 * time.Second,
				Timeout:     5000 * time.Second,
				StartPeriod: 0 * time.Second,
				Retries:     retries,
			},
		})
	}

	for _, testData := range testHealthCheckData {
		res, healthConf, err := deserialiseHealtheckInstruction(testData.input)
		require.NoError(t, err, "No error should occur")
		assert.Equal(t, &testData.expectedHealthConf, healthConf)
		assert.Equal(t, testData.reconstructedHealthcheck, res)
	}
}
