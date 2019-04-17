package pdiscover

// stubs, so 'go get' doesn't show errors on Macs...

func createListener() (eventListener, error) {
	return nil, nil
}

func (w *Watcher) unregister(pid int) error {
	return nil
}

func (w *Watcher) register(pid int, flags uint32) error {
	return nil
}

func (w *Watcher) readEvents() {
}

func (w *Watcher) readAllEvents() {
}

func (w *Watcher) isWatching(pid int, event uint32) bool {
	return false
}
