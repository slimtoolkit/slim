// Package parser implements a Dockerfile parser
package parser

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/slimtoolkit/slim/pkg/docker/dockerfile/ast"
	"github.com/slimtoolkit/slim/pkg/docker/dockerfile/spec"
	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

var (
	ErrInvalidDockerfile = errors.New("invalid Dockerfile")
)

//TODO:
//* support incremental, partial and instruction level parsing
//* support parsing from reader and from string

func FromFile(fpath string) (*spec.Dockerfile, error) {
	fo, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}

	defer fo.Close()

	astParsed, err := ast.Parse(fo)
	if err != nil {
		return nil, err
	}

	if !astParsed.AST.IsValid {
		return nil, ErrInvalidDockerfile
	}

	dockerfile := spec.NewDockerfile()
	dockerfile.Name = filepath.Base(fpath)
	dockerfile.Location = filepath.Dir(fpath)
	dockerfile.Lines = astParsed.Lines

	if astParsed.AST.StartLine > -1 && len(astParsed.AST.Children) > 0 {
		var currentStage *spec.BuildStage
		instStageIndex := -1
		for idx, node := range astParsed.AST.Children {
			inst := &instruction.Field{
				GlobalIndex: idx,
				StageIndex:  instStageIndex,
				IsValid:     node.IsValid,
				RawData:     node.Original,
				StartLine:   node.StartLine,
				EndLine:     node.EndLine,
				Name:        node.Value,
				Flags:       node.Flags,
				ArgsRaw:     node.ArgsRaw,
			}

			if len(dockerfile.Lines) > 0 &&
				inst.StartLine > 0 &&
				inst.StartLine <= len(dockerfile.Lines) &&
				inst.EndLine <= len(dockerfile.Lines) &&
				inst.EndLine >= inst.StartLine {
				inst.RawLines = dockerfile.Lines[inst.StartLine-1 : inst.EndLine]
			}

			if !inst.IsValid {
				inst.Errors = append(inst.Errors, node.Errors...)
			}

			if inst.Name == instruction.Onbuild &&
				node.Next != nil &&
				len(node.Next.Children) > 0 {
				inst.IsOnBuild = true
				node = node.Next.Children[0]
				inst.Name = node.Value
				inst.ArgsRaw = node.ArgsRaw
				inst.Flags = node.Flags
			}

			for n := node.Next; n != nil; n = n.Next {
				inst.Args = append(inst.Args, n.Value)
			}

			if _, ok := node.Attributes["json"]; ok {
				inst.IsJSONForm = true
			}

			if inst.Name == instruction.From {
				currentStage = spec.NewBuildStage()
				currentStage.FromInstruction = inst
				currentStage.Index = len(dockerfile.Stages)
				instStageIndex = -1

				currentStage.StartLine = inst.StartLine

				if len(inst.Args) == 3 && strings.ToLower(inst.Args[1]) == "as" {
					currentStage.Name = inst.Args[2]
					dockerfile.StagesByName[currentStage.Name] = currentStage
				}

				if len(inst.Args) > 0 {
					var parts []string
					var hasDigest bool
					switch {
					case strings.Contains(inst.Args[0], ":"):
						parts = strings.Split(inst.Args[0], ":")
					case strings.Contains(inst.Args[0], "@"):
						parts = strings.Split(inst.Args[0], "@")
						hasDigest = true
					default:
						parts = append(parts, inst.Args[0])
					}

					if len(parts) > 0 {
						if len(parts[0]) > 0 {
							if strings.HasPrefix(parts[0], "$") {
								argName := GetRefName(parts[0])
								argVal, ok := dockerfile.FromArgs[argName]
								if !ok {
									currentStage.UnknownFromArgs[argName] = struct{}{}
								} else {
									currentStage.FromArgs[argName] = argVal
								}

								if len(parts) == 1 {
									if len(argVal) > 0 {
										switch {
										case strings.Contains(argVal, ":"):
											parts = strings.Split(argVal, ":")
										case strings.Contains(argVal, "@"):
											parts = strings.Split(argVal, "@")
											hasDigest = true
										default:
											parts = nil
											parts = append(parts, argVal)
										}

										if len(parts) == 1 {
											currentStage.Parent.Name = parts[0]
										}
									}
									currentStage.Parent.BuildArgAll = argName
								} else {
									currentStage.Parent.BuildArgName = argName
									currentStage.Parent.Name = argVal
								}
							} else {
								currentStage.Parent.Name = parts[0]
							}
						} else {
							currentStage.Parent.HasEmptyName = true
						}
					}

					if len(parts) == 2 {
						if len(parts[1]) > 0 {
							if strings.HasPrefix(parts[1], "$") {
								argName := GetRefName(parts[1])
								argVal, ok := dockerfile.FromArgs[argName]
								if !ok {
									currentStage.UnknownFromArgs[argName] = struct{}{}
								} else {
									currentStage.FromArgs[argName] = argVal
								}

								if hasDigest {
									currentStage.Parent.BuildArgDigest = argName
									currentStage.Parent.Digest = argVal
								} else {
									currentStage.Parent.BuildArgTag = argName
									currentStage.Parent.Tag = argVal
								}
							} else {
								if hasDigest {
									currentStage.Parent.Digest = parts[1]
								} else {
									currentStage.Parent.Tag = parts[1]
								}
							}
						} else {
							if hasDigest {
								currentStage.Parent.HasEmptyDigest = true
							} else {
								currentStage.Parent.HasEmptyTag = true
							}
						}
					}

					if currentStage.Parent.Name != "" {
						if parentStage, ok := dockerfile.StagesByName[currentStage.Parent.Name]; ok {
							parentStage.IsUsed = true
							currentStage.Parent.ParentStage = parentStage
							currentStage.StageReferences[currentStage.Parent.Name] = parentStage
						}
					}
				} else {
					currentStage.Parent.HasEmptyName = true
					currentStage.Parent.HasEmptyTag = true
					currentStage.Parent.HasEmptyDigest = true
				}

				dockerfile.Stages = append(dockerfile.Stages, currentStage)
				dockerfile.LastStage = currentStage
			}

			dockerfile.AllInstructions = append(dockerfile.AllInstructions, inst)
			dockerfile.InstructionsByType[inst.Name] = append(
				dockerfile.InstructionsByType[inst.Name], inst)

			if currentStage != nil {
				currentStage.EndLine = inst.EndLine

				inst.StageID = currentStage.Index
				instStageIndex++
				inst.StageIndex = instStageIndex
				currentStage.AllInstructions = append(currentStage.AllInstructions, inst)

				if inst.Name == instruction.Onbuild {
					currentStage.OnBuildInstructions = append(currentStage.OnBuildInstructions, inst)
				} else if inst.Name == instruction.Copy {
					for _, flag := range inst.Flags {
						if strings.HasPrefix(flag, "--from=") {
							fparts := strings.SplitN(flag, "=", 2)
							//possible values: stage index, stage name, image name
							var stageRef *spec.BuildStage
							refIdx, err := strconv.Atoi(fparts[1])
							if err == nil {
								if refIdx >= 0 && refIdx < len(dockerfile.Stages) {
									stageRef = dockerfile.Stages[refIdx]
								}
							} else {
								stageRef = dockerfile.StagesByName[fparts[1]]
							}

							if stageRef != nil {
								stageRef.IsUsed = true
								currentStage.StageReferences[fparts[1]] = stageRef
							} else {
								currentStage.ExternalReferences[fparts[1]] = struct{}{}
								//todo: check if it references a local (or remote) image
							}
						}
					}
				} else if instruction.IsKnown(inst.Name) {
					currentStage.CurrentInstructions = append(currentStage.CurrentInstructions, inst)
					currentStage.CurrentInstructionsByType[inst.Name] = append(
						currentStage.CurrentInstructionsByType[inst.Name], inst)

					if inst.IsValid {
						switch inst.Name {
						case instruction.Arg:
							for _, iarg := range inst.Args {
								if iarg == "" {
									continue
								}

								if strings.Contains(iarg, "=") {
									iaparts := strings.SplitN(iarg, "=", 2)
									currentStage.BuildArgs[iaparts[0]] = iaparts[1]
								} else {
									currentStage.BuildArgs[iarg] = ""
								}
							}
							//only one ARG is supposed to be defined, but we'll use all
							//the 'ARG' value count lint check will detect the extra values
							//the k=v ARG values are also not parsed (unlike ENV k=v values)
						case instruction.Env:
							for i := 0; i < len(inst.Args) && (i+1) < len(inst.Args); i += 2 {
								if len(inst.Args[i]) == 0 {
									continue
								}

								currentStage.EnvVars[inst.Args[i]] = inst.Args[i+1]
							}
						}
					}
				}
			} else {
				if inst.Name == instruction.Arg {
					if inst.IsValid && len(inst.Args) > 0 {
						dockerfile.ArgInstructions = append(dockerfile.ArgInstructions, inst)
						parts := strings.Split(inst.Args[0], "=")
						argVal := ""
						if len(parts) == 2 {
							argVal = parts[1]
						}

						dockerfile.FromArgs[parts[0]] = argVal
					} //note: should have a lint check for ARG without 'args'
				} else {
					dockerfile.StagelessInstructions = append(dockerfile.StagelessInstructions, inst)
				}
			}

			if !inst.IsValid {
				if !instruction.IsKnown(inst.Name) {
					dockerfile.UnknownInstructions = append(dockerfile.UnknownInstructions, inst)
					if currentStage != nil {
						currentStage.UnknownInstructions = append(currentStage.UnknownInstructions, inst)
					}
				} else {
					dockerfile.InvalidInstructions = append(dockerfile.InvalidInstructions, inst)
					if currentStage != nil {
						currentStage.InvalidInstructions = append(currentStage.InvalidInstructions, inst)
					}
				}
			}
		}

		if dockerfile.LastStage != nil {
			dockerfile.LastStage.IsUsed = true
		}
	}

	dockerfile.Warnings = astParsed.Warnings

	return dockerfile, nil
}

func GetRefName(ref string) string {
	ref = strings.TrimPrefix(ref, "$")
	if strings.HasPrefix(ref, "{") && strings.HasSuffix(ref, "}") {
		ref = strings.TrimPrefix(ref, "{")
		ref = strings.TrimSuffix(ref, "}")
	}

	return ref
}
