package main

import (
	"internal/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/pdiscover"
)

//Process Event Monitor goal:
//Watch the processes to separate the activity we care about from unrelated stuff running in the background.

type peMonitorReport struct {
	Children map[int][]int
	Parents  map[int]int
}

func peRunMonitor(stopChan chan struct{}) <-chan *peMonitorReport {
	log.Info("pemon: starting...")

	watcher, err := pdiscover.NewAllWatcher(pdiscover.PROC_EVENT_ALL)
	utils.FailOn(err)

	reportChan := make(chan *peMonitorReport, 1)

	go func() {
		report := &peMonitorReport{
			Children: make(map[int][]int),
			Parents:  make(map[int]int),
		}

	done:
		for {
			select {
			case <-stopChan:
				log.Info("pemon: stopping...")
				break done
			case ev := <-watcher.Fork:
				report.Children[ev.ParentPid] = append(report.Children[ev.ParentPid], ev.ChildPid)
				report.Parents[ev.ChildPid] = ev.ParentPid
			case <-watcher.Exec:
			case <-watcher.Exit:
			case err := <-watcher.Error:
				utils.FailOn(err)
			}
		}

		reportChan <- report
		watcher.Close()
	}()

	return reportChan
}
