package apparmor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"text/template"

	"slim/report"
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
func GenProfile(artifactLocation string, profileName string) error {
	containerReportFileName := "creport.json"
	containerReportFilePath := filepath.Join(artifactLocation, containerReportFileName)

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
		if aprops.Flags["X"] {
			profileData.ExeFileRules = append(profileData.ExeFileRules,
				appArmorFileRule{
					FilePath: aprops.FilePath,
					PermSet:  report.PermSetFromFlags(aprops.Flags),
				})
		} else if aprops.Flags["W"] {
			profileData.WriteFileRules = append(profileData.WriteFileRules,
				appArmorFileRule{
					FilePath: aprops.FilePath,
					PermSet:  report.PermSetFromFlags(aprops.Flags),
				})
		} else if aprops.Flags["R"] {
			profileData.ReadFileRules = append(profileData.ReadFileRules,
				appArmorFileRule{
					FilePath: aprops.FilePath,
					PermSet:  report.PermSetFromFlags(aprops.Flags),
				})
		} else {
			//logrus.Printf("docker-slim: genAppArmorProfile - other artifact => %v\n", aprops)
			//note: most are Symlinks
			//&{Symlink /lib/x86_64-linux-gnu/libc.so.6 ---------- Lrwxrwxrwx libc-2.19.so map[]  12  }
			//todo: double check this file:
			//&{File /etc/ld.so.cache ---------- -rw-r--r--  map[] data 15220 ca4491d92fac4500148a18bd9cada91b49e08701 }
			//-rw-r--r--  1 user  group    15K Month  1 20:14 ld.so.cache
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
