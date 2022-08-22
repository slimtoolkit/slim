package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/docker-slim/docker-slim/pkg/consts"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
)

type ExecutionContext struct {
	Out             *Output
	cleanupHandlers []func()
}

func (ref *ExecutionContext) Exit(exitCode int) {
	ref.doCleanup()
	ref.exit(exitCode)
}

func (ref *ExecutionContext) AddCleanupHandler(handler func()) {
	if handler != nil {
		ref.cleanupHandlers = append(ref.cleanupHandlers, handler)
	}
}

func (ref *ExecutionContext) doCleanup() {
	if len(ref.cleanupHandlers) == 0 {
		return
	}

	//call cleanup handlers in reverse order
	for i := len(ref.cleanupHandlers) - 1; i >= 0; i-- {
		cleanup := ref.cleanupHandlers[i]
		if cleanup != nil {
			cleanup()
		}
	}
}

func (ref *ExecutionContext) FailOn(err error) {
	if err != nil {
		ref.doCleanup()
	}

	errutil.FailOn(err)
}

func (ref *ExecutionContext) exit(exitCode int) {
	ShowCommunityInfo(ref.Out.JSONFlag)
	os.Exit(exitCode)
}

func NewExecutionContext(cmdName, jsonFlag string) *ExecutionContext {
	ref := &ExecutionContext{
		Out: NewOutput(cmdName, jsonFlag),
	}

	return ref
}

type Output struct {
	CmdName  string
	JSONFlag string
}

func NewOutput(cmdName, jsonFlag string) *Output {
	ref := &Output{
		CmdName:  cmdName,
		JSONFlag: jsonFlag,
	}

	return ref
}

func NoColor() {
	color.NoColor = true
}

type OutVars map[string]interface{}

func (ref *Output) LogDump(logType, data string, params ...OutVars) {
	var info string
	if len(params) > 0 {
		kvSet := params[0]
		if len(kvSet) > 0 {
			var builder strings.Builder
			for k, v := range kvSet {
				builder.WriteString(kcolor(k))
				builder.WriteString("=")
				builder.WriteString(fmt.Sprintf("'%s'", vcolor("%v", v)))
				builder.WriteString(" ")
			}

			info = builder.String()
		}
	}

	fmt.Printf("cmd=%s log='%s' event=LOG.START %s ====================\n", ref.CmdName, logType, info)
	fmt.Println(data)
	fmt.Printf("cmd=%s log='%s' event=LOG.END %s ====================\n", ref.CmdName, logType, info)
}

func (ref *Output) Prompt(data string) {
	color.Set(color.FgHiRed)
	defer color.Unset()

	if ref.JSONFlag == "json" {
		//marshal data to json
		var jsonData []byte
		if len(data) > 0 {
			msg := map[string]string{
				"cmd":    ref.CmdName,
				"prompt": data,
			}
			jsonData, _ = json.Marshal(msg)
			fmt.Println(string(jsonData))
		}
	} else {
		fmt.Printf("cmd=%s prompt='%s'\n", ref.CmdName, data)
	}

}

func (ref *Output) Error(errType string, data string) {
	color.Set(color.FgHiRed)
	defer color.Unset()

	if ref.JSONFlag == "json" {
		//marshal data to json
		var jsonData []byte
		if len(data) > 0 {
			msg := map[string]string{
				"cmd":     ref.CmdName,
				"error":   errType,
				"message": data,
			}
			jsonData, _ = json.Marshal(msg)
			fmt.Println(string(jsonData))
		}
	} else {
		fmt.Printf("cmd=%s error=%s message='%s'\n", ref.CmdName, errType, data)
	}

}

func (ref *Output) Message(data string) {
	color.Set(color.FgHiMagenta)
	defer color.Unset()

	if ref.JSONFlag == "json" {
		//marshal data to json
		var jsonData []byte
		if len(data) > 0 {
			msg := map[string]string{
				"cmd":     ref.CmdName,
				"message": data,
			}
			jsonData, _ = json.Marshal(msg)
			fmt.Println(string(jsonData))
		}
	} else {
		fmt.Printf("cmd=%s message='%s'\n", ref.CmdName, data)
	}

}

func (ref *Output) State(state string, params ...OutVars) {
	var exitInfo string
	var info string
	var sep string

	if len(params) > 0 {
		var minCount int
		kvSet := params[0]
		if exitCode, ok := kvSet["exit.code"]; ok {
			minCount = 1
			exitInfo = fmt.Sprintf(" code=%d", exitCode)
		}

		if len(kvSet) > minCount {
			var builder strings.Builder
			sep = " "

			for k, v := range kvSet {
				if k == "exit.code" {
					continue
				}

				builder.WriteString(k)
				builder.WriteString("=")
				val := fmt.Sprintf("%v", v)
				if strings.Contains(val, " ") && !strings.HasPrefix(val, `"`) {
					val = fmt.Sprintf("\"%s\"", val)
				}

				builder.WriteString(val)
				builder.WriteString(" ")
			}

			info = builder.String()
		}
	}

	if state == "exited" || strings.Contains(state, "error") {
		color.Set(color.FgHiRed, color.Bold)
	} else {
		color.Set(color.FgCyan, color.Bold)
	}
	defer color.Unset()

	//marshal info to json
	if ref.JSONFlag == "json" {
		var jsonData []byte
		var jsonInfoData []byte
		if len(info) > 0 {
			jsonInfoData, _ = json.Marshal(params[0])
			msg := map[string]string{
				"cmd":       ref.CmdName,
				"state":     state,
				"exit.info": exitInfo,
				"info":      string(jsonInfoData),
			}
			jsonData, _ = json.Marshal(msg)
			fmt.Println(string(jsonData))
		}
	} else {
		fmt.Printf("cmd=%s state=%s%s%s%s\n", ref.CmdName, state, exitInfo, sep, info)
	}
}

var (
	itcolor = color.New(color.FgMagenta, color.Bold).SprintFunc()
	kcolor  = color.New(color.FgHiGreen, color.Bold).SprintFunc()
	vcolor  = color.New(color.FgHiBlue).SprintfFunc()
)

func (ref *Output) Info(infoType string, params ...OutVars) {
	var data string
	var sep string

	if len(params) > 0 {
		kvSet := params[0]
		if len(kvSet) > 0 {
			var builder strings.Builder
			sep = " "

			for k, v := range kvSet {
				builder.WriteString(kcolor(k))
				builder.WriteString("=")
				builder.WriteString(fmt.Sprintf("'%s'", vcolor("%v", v)))
				builder.WriteString(" ")
			}

			data = builder.String()
		}
	}

	switch ref.JSONFlag {
	case "json":
		var jsonData []byte
		var jsonInfoData []byte
		if len(data) > 0 {
			jsonInfoData, _ = json.Marshal(params[0])
			msg := map[string]string{
				"cmd":  ref.CmdName,
				"info": infoType,
				"data": string(jsonInfoData),
			}
			jsonData, _ = json.Marshal(msg)
			fmt.Println(string(jsonData))
		}
	case "text":
		fmt.Printf("cmd=%s info=%s%s%s\n", ref.CmdName, itcolor(infoType), sep, data)

	default:
		fmt.Printf("Unknown json flag: %s\n", ref.JSONFlag)
	}

}

func ShowCommunityInfo(jsonFlag string) {

	type Data struct {
		App     string `json:"app"`
		Message string `json:"message"`
		Info    string `json:"info"`
	}

	type CommunityInfo struct {
		Data []Data `json:"data"`
	}

	var data Data
	var community CommunityInfo

	color.Set(color.FgHiMagenta)
	defer color.Unset()

	data.App = "docker-slim"
	data.Message = "Join the Gitter channel to ask questions or to share your feedback"
	data.Info = consts.CommunityGitter

	community.Data = append(community.Data, data)

	data.App = "docker-slim"
	data.Message = "Join the Discord server to ask questions or to share your feedback"
	data.Info = consts.CommunityDiscord

	community.Data = append(community.Data, data)

	data.App = "docker-slim"
	data.Message = "GitHub Discussions"
	data.Info = consts.CommunityDiscussions

	community.Data = append(community.Data, data)

	if jsonFlag == "json" {
		var jsonData []byte
		jsonData, _ = json.Marshal(community.Data)
		fmt.Println(string(jsonData))
	} else {
		for _, v := range community.Data {
			fmt.Printf("'app':'%s' 'message':'%s' 'info':'%s'\n", v.App, v.Message, v.Info)
		}
	}
}
