//go:build linux
// +build linux

package pevent

import (
	"github.com/slimtoolkit/slim/pkg/pdiscover"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"

	log "github.com/sirupsen/logrus"
)

//Process Event Monitor goal:
//Watch the processes to separate the activity we care about from unrelated stuff running in the background.

// Run starts the PEVENT monitor
func Run(stopChan <-chan struct{}) <-chan *report.PeMonitorReport {
	log.Info("pemon: starting...")

	//"connection refused" with boot2docker...
	watcher, err := pdiscover.NewAllWatcher(pdiscover.PROC_EVENT_ALL)
	errutil.FailOn(err)

	reportChan := make(chan *report.PeMonitorReport, 1)

	go func() {
		peReport := &report.PeMonitorReport{
			Children: map[int][]int{},
			Parents:  map[int]int{},
		}

	done:
		for {
			select {
			case <-stopChan:
				log.Info("pemon: stopping...")
				break done
			case ev := <-watcher.Fork:
				peReport.Children[ev.ParentPid] = append(peReport.Children[ev.ParentPid], ev.ChildPid)
				peReport.Parents[ev.ChildPid] = ev.ParentPid
			case <-watcher.Exec:
			case <-watcher.Exit:
			case err := <-watcher.Error:
				errutil.FailOn(err)
			}
		}

		reportChan <- peReport
		watcher.Close()
	}()

	return reportChan
}
