// Package check contains the linter checks
package check

import (
	"fmt"
	"strings"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &PyPipInstallLatest{
		Info: Info{
			ID:           "ID.20018",
			Name:         "Python Pip installs latest package version",
			Description:  "Python Pip installs latest package version",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20018",
			MainMessage:  "Python Pip installs latest package version",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d package=%s",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type PyPipInstallLatest struct {
	Info
}

func (c *PyPipInstallLatest) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if instructions, ok := stage.CurrentInstructionsByType[instruction.Run]; ok {
			for _, inst := range instructions {
				if len(inst.ArgsRaw) == 0 {
					continue
				}

				var args []string
				if inst.IsJSONForm {
					args = inst.Args
				} else {
					var err error
					args, err = shlex.Split(inst.ArgsRaw)
					if err != nil {
						log.Fatal(err)
					}
				}

				if len(args) > 1 {
					cmdName := strings.ToLower(args[0])

					if strings.HasPrefix(cmdName, "python") &&
						len(args) > 3 &&
						args[1] == "-m" {
						cmdName = strings.ToLower(args[2])
						args = args[2:]
						//handle 'pythonX -m pip install pkgname'
					}

					//pip install -x 'pkgname>=x.y.z,<a.b.c' pkgname==x.y --somefield

					if strings.HasPrefix(cmdName, "pip") &&
						strings.ToLower(args[1]) == "install" {
						if len(args) == 4 &&
							(args[2] == "-U" || args[2] == "--upgrade") &&
							args[3] == "pip" {
							continue
						}

						if strings.Contains(inst.ArgsRaw, " -r ") ||
							strings.Contains(inst.ArgsRaw, " --requirement") ||
							strings.Contains(inst.ArgsRaw, " -c ") ||
							strings.Contains(inst.ArgsRaw, " --constraint") {
							continue
						}

						//assumption: requirements/constraint files contain versions
						//todo:
						//should confirm the requirements/constraint files
						//and what versions they include

						if (strings.Contains(inst.ArgsRaw, " git") ||
							strings.Contains(inst.ArgsRaw, "https://") ||
							strings.Contains(inst.ArgsRaw, "http://")) &&
							strings.Contains(inst.ArgsRaw, "@") {
							continue
						}

						for i := 2; i < len(args); i++ {
							if strings.HasPrefix(args[i], "-") {
								continue
							}

							//todo: check for local packages and wheels...

							pkgInfoParts := strings.FieldsFunc(args[i],
								func(c rune) bool {
									return c == '=' || c == '<' || c == '>' || c == '!'
								})

							pkgName := pkgInfoParts[0]
							if pkgName[0] == '\'' || pkgName[0] == '"' {
								pkgName = pkgName[1:]
							}

							if strings.Contains(args[i], "==") ||
								strings.Contains(args[i], "<=") ||
								strings.Contains(args[i], "<") {
								continue
							}

							if !result.Hit {
								result.Hit = true
								result.Message = c.MainMessage
							}

							match := &Match{
								Stage:       stage,
								Instruction: inst,
								Message: fmt.Sprintf(c.MatchMessage,
									inst.StartLine,
									inst.EndLine,
									inst.GlobalIndex,
									inst.StageID,
									inst.StageIndex,
									pkgInfoParts[0]),
							}

							result.Matches = append(result.Matches, match)
						}
					}
				}
			}
		}
	}

	return result, nil
}
