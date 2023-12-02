package appbom

import (
	//log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/appbom"
)

const appName = command.AppName

type ovars = app.OutVars

// OnCommand implements the 'server' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams) {
	//logger := log.WithFields(log.Fields{"app": appName, "command": Name})

	info := appbom.Get()
	if info == nil {
		xc.Out.Error("appbom.info", "missing")
		xc.Exit(0)
	}

	if info.BuilderHash != "" {
		xc.Out.Info("appbom", ovars{"builder_hash": info.BuilderHash})
	}

	xc.Out.Info("appbom", ovars{"runtime": info.Runtime})
	xc.Out.Info("appbom.entrypoint",
		ovars{
			"path":    info.Entrypoint.Path,
			"version": info.Entrypoint.Version,
		})

	if info.SourceControl != nil {
		xc.Out.Info("appbom.source_control",
			ovars{
				"type":              info.SourceControl.Type,
				"revision":          info.SourceControl.Revision,
				"revision_time":     info.SourceControl.RevisionTime,
				"has_local_changes": info.SourceControl.HasLocalChanges,
			})
	}

	outputParam := func(param *appbom.ParamInfo, header string) {
		if param != nil {
			xc.Out.Info(header,
				ovars{
					"name":  param.Name,
					"type":  param.Type,
					"value": param.Value,
				})
		}
	}

	outputParam(info.BuildParams.Os, "appbom.build_params.os")
	outputParam(info.BuildParams.Arch, "appbom.build_params.arch")
	outputParam(info.BuildParams.ArchFeature, "appbom.build_params.arch_feature")
	outputParam(info.BuildParams.BuildMode, "appbom.build_params.build_mode")
	outputParam(info.BuildParams.Compiler, "appbom.build_params.compiler")
	outputParam(info.BuildParams.CgoEnabled, "appbom.build_params.cgo_enabled")
	outputParam(info.BuildParams.CgoCFlags, "appbom.build_params.cgo_cflags")
	outputParam(info.BuildParams.CgoCppFlags, "appbom.build_params.cgo_cppflags")
	outputParam(info.BuildParams.CgoCxxFlags, "appbom.build_params.cgo_cxxflags")
	outputParam(info.BuildParams.CgoLdFlags, "appbom.build_params.cgo_ldflags")

	if len(info.OtherParams) > 0 {
		for k, v := range info.OtherParams {
			xc.Out.Info("appbom.other_params", ovars{"key": k, "value": v})
		}
	}

	if len(info.Includes) > 0 {
		for _, v := range info.Includes {
			vals := ovars{
				"name":    v.Name,
				"version": v.Version,
				"path":    v.Path,
				"hash":    v.Hash,
			}
			if v.ReplacedBy != "" {
				vals["replaced_by"] = v.ReplacedBy
			}

			xc.Out.Info("appbom.includes", vals)
		}
	}
}
