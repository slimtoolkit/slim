// +build windows

package prompt

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func (p *Prompt) handleSignals(exitCh chan int, winSizeCh chan *WinSize, stop chan struct{}) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(
		sigCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	for {
		select {
		case <-stop:
			log.Println("[INFO] stop handleSignals")
			return
		case s := <-sigCh:
			switch s {

			case syscall.SIGINT: // kill -SIGINT XXXX or Ctrl+c
				log.Println("[SIGNAL] Catch SIGINT")
				exitCh <- 0

			case syscall.SIGTERM: // kill -SIGTERM XXXX
				log.Println("[SIGNAL] Catch SIGTERM")
				exitCh <- 1

			case syscall.SIGQUIT: // kill -SIGQUIT XXXX
				log.Println("[SIGNAL] Catch SIGQUIT")
				exitCh <- 0
			}
		}
	}
}
