package apparmor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"text/template"

	"github.com/slimtoolkit/slim/pkg/report"
)

const appArmorTemplate = `
profile {{.ProfileName}} flags=(attach_disconnected,mediate_deleted) {

  network,

{{range $value := .ExeFileRules}}  {{$value.FilePath}} {{$value.PermSet}},
{{end}}
{{range $value := .WriteFileRules}}  {{$value.FilePath}} {{$value.PermSet}},
{{end}}
{{range $value := .ReadFileRules}}  {{$value.FilePath}} {{$value.PermSet}},
{{end}}
}
`

type appArmorFileRule struct {
	FilePath string
	PermSet  string
}

type appArmorProfileData struct {
	ProfileName    string
	ExeFileRules   []appArmorFileRule
	WriteFileRules []appArmorFileRule
	ReadFileRules  []appArmorFileRule
}

//TODO:
//need to safe more metadata about the artifacts in the monitor data
//1. exe bit
//2. w/r operation info (so we can add useful write rules)

// GenProfile creates an AppArmor profile
func GenProfile(artifactLocation string, profileName string) error {
	containerReportFilePath := filepath.Join(artifactLocation, report.DefaultContainerReportFileName)

	if _, err := os.Stat(containerReportFilePath); err != nil {
		return err
	}
	reportFile, err := os.Open(containerReportFilePath)
	if err != nil {
		return err
	}
	defer reportFile.Close()

	var creport report.ContainerReport
	if err = json.NewDecoder(reportFile).Decode(&creport); err != nil {
		return err
	}

	profilePath := filepath.Join(artifactLocation, profileName)

	profileFile, err := os.OpenFile(profilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	defer profileFile.Close()

	profileData := appArmorProfileData{ProfileName: profileName}

	for _, aprops := range creport.Image.Files {
		if aprops == nil {
			continue
		}
		if aprops.Flags == nil {
			//default to "R" (todo: double check flag creation...)
			profileData.ReadFileRules = append(profileData.ReadFileRules,
				appArmorFileRule{
					FilePath: aprops.FilePath,
					PermSet:  "r",
				})
		} else {
			switch {
			case aprops.Flags["X"]:
				profileData.ExeFileRules = append(profileData.ExeFileRules,
					appArmorFileRule{
						FilePath: aprops.FilePath,
						PermSet:  report.PermSetFromFlags(aprops.Flags),
					})
			case aprops.Flags["W"]:
				profileData.WriteFileRules = append(profileData.WriteFileRules,
					appArmorFileRule{
						FilePath: aprops.FilePath,
						PermSet:  report.PermSetFromFlags(aprops.Flags),
					})
			case aprops.Flags["R"]:
				profileData.ReadFileRules = append(profileData.ReadFileRules,
					appArmorFileRule{
						FilePath: aprops.FilePath,
						PermSet:  report.PermSetFromFlags(aprops.Flags),
					})
			default:
				//logrus.Printf("slim: genAppArmorProfile - other artifact => %v\n", aprops)
				//note: most are Symlinks
			}
		}
	}

	t, err := template.New("profile").Parse(appArmorTemplate)
	if err != nil {
		return err
	}

	if err := t.Execute(profileFile, profileData); err != nil {
		return err
	}

	return nil
}
