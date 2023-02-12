package reverse

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

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

func TestProcessRawInst(t *testing.T) {
	t.Run("when the instruction is in args format", func(t *testing.T) {
		t.Run("but does not have arguments", func(t *testing.T) {
			raw := "|/bin/sh"

			t.Run("it treats the instruction as a pure RUN", func(t *testing.T) {
				expected := "RUN [\"|/bin/sh\"]\n"
				result, _ := processRawInst(raw)

				assert.Equal(t, expected, result)
			})

			t.Run("it returns true as the exec form", func(t *testing.T) {
				_, ief := processRawInst(raw)

				assert.True(t, ief)
			})

			t.Run("it logs the bad part count", func(t *testing.T) {
				logged := withFakeLogger(func() {
					processRawInst(raw)
				})

				assert.True(t, logged.Match("unexpected number of parts"))
			})
		})

		t.Run("but contains bad shell syntax", func(t *testing.T) {
			raw := "|1 /bin/sh -c echo '"

			t.Run("it treats the instruction as a pure RUN", func(t *testing.T) {
				expected := "RUN |1 /bin/sh -c echo '"
				result, _ := processRawInst(raw)

				assert.Equal(t, expected, result)
			})

			t.Run("it returns true as the exec form", func(t *testing.T) {
				_, ief := processRawInst(raw)

				assert.True(t, ief)
			})

			t.Run("it logs the bad command", func(t *testing.T) {
				logged := withFakeLogger(func() {
					processRawInst(raw)
				})

				assert.True(t, logged.Match("malformed - "))
			})
		})

		t.Run("but contains too few args", func(t *testing.T) {
			raw := "|4 /bin/sh -c echo foo"

			t.Run("it treats the instruction as a pure RUN", func(t *testing.T) {
				expected := "RUN [\"|4\",\"/bin/sh\",\"-c\",\"echo\",\"foo\"]\n"
				result, _ := processRawInst(raw)

				assert.Equal(t, expected, result)
			})

			t.Run("it returns true as the exec form", func(t *testing.T) {
				_, ief := processRawInst(raw)

				assert.True(t, ief)
			})

			t.Run("it logs the bad command", func(t *testing.T) {
				logged := withFakeLogger(func() {
					processRawInst(raw)
				})

				assert.True(t, logged.Match("malformed - "))
			})
		})

		t.Run("but does not have an argnumment number", func(t *testing.T) {
			raw := "|/bin/sh -c echo 'blah'"

			t.Run("it treats the instruction as a pure RUN", func(t *testing.T) {
				expected := "RUN [\"|/bin/sh\",\"-c\",\"echo\",\"blah\"]\n"
				result, _ := processRawInst(raw)

				assert.Equal(t, expected, result)
			})

			t.Run("it returns true as the exec form", func(t *testing.T) {
				_, ief := processRawInst(raw)

				assert.True(t, ief)
			})

			t.Run("it logs the bad arg count", func(t *testing.T) {
				logged := withFakeLogger(func() {
					processRawInst(raw)
				})

				assert.True(t, logged.Match("malformed number of ARGs"))
			})
		})

		t.Run("and is a well-formed shell format instruction", func(t *testing.T) {
			raw := "|0 /bin/sh -c echo 'blah'"

			t.Run("it processes the instruction as a shell command", func(t *testing.T) {
				expected := "RUN echo blah"
				result, _ := processRawInst(raw)

				assert.Equal(t, expected, result)
			})

			t.Run("it returns false as the exec form", func(t *testing.T) {
				_, ief := processRawInst(raw)

				assert.False(t, ief)
			})
		})

		t.Run("and is a well-formed raw instruction", func(t *testing.T) {
			raw := "|0 echo 'blah'"

			t.Run("it processes the instruction as an exec command", func(t *testing.T) {
				expected := "RUN [\"echo\",\"blah\"]\n"
				result, _ := processRawInst(raw)

				assert.Equal(t, expected, result)
			})

			t.Run("it returns true as the exec form", func(t *testing.T) {
				_, ief := processRawInst(raw)

				assert.True(t, ief)
			})
		})
	})

	t.Run("when the instruction has an uncaught instruction prefix", func(t *testing.T) {
		raw := "EXPOSE 8675"

		t.Run("it passes the instruction back unchanged", func(t *testing.T) {
			result, _ := processRawInst(raw)

			assert.Equal(t, raw, result)
		})

		t.Run("it returns false as the exec form", func(t *testing.T) {
			_, ief := processRawInst(raw)

			assert.False(t, ief)
		})
	})

	t.Run("when the instruction is raw text", func(t *testing.T) {
		raw := "foo bar"

		t.Run("it is treated as an exec format RUN", func(t *testing.T) {
			expected := "RUN [\"foo\",\"bar\"]\n"
			result, _ := processRawInst(raw)

			assert.Equal(t, expected, result)
		})

		t.Run("it returns true as the exec form", func(t *testing.T) {
			_, ief := processRawInst(raw)

			assert.True(t, ief)
		})
	})

}

type fakeLogger struct {
	Lines []string
}

func (l *fakeLogger) Write(msg []byte) (int, error) {
	if l.Lines == nil {
		l.Lines = []string{}
	}

	l.Lines = append(l.Lines, string(msg))

	return len(msg), nil
}

func (l *fakeLogger) Match(substr string) bool {
	for _, line := range l.Lines {
		if strings.Contains(line, substr) {
			return true
		}
	}

	return false
}

func withFakeLogger(proc func()) *fakeLogger {
	logger := &fakeLogger{}
	log.SetOutput(logger)

	proc()

	log.SetOutput(os.Stderr)
	return logger
}
