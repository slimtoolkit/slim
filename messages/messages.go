package messages

import (
	"encoding/json"
	"errors"
)

var (
	ErrUnknownMessage = errors.New("unknown type")
)

type MessageName string

const (
	StartMonitorName MessageName = "cmd.monitor.start"
	StopMonitorName  MessageName = "cmd.monitor.stop"
)

type Message interface {
	GetName() MessageName
}

type StartMonitor struct {
	AppName  string   `json:"app_name"`
	AppArgs  []string `json:"app_args,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
	Includes []string `json:"includes,omitempty"`
}

func (m *StartMonitor) GetName() MessageName {
	return StartMonitorName
}

type StopMonitor struct {
}

func (m *StopMonitor) GetName() MessageName {
	return StopMonitorName
}

type messageWrapper struct {
	Name MessageName     `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

func Encode(m Message) ([]byte, error) {
	obj := messageWrapper{
		Name: m.GetName(),
	}
	
	switch v := m.(type) {
		case *StartMonitor:
			var err error
			obj.Data, err = json.Marshal(v)
			if err != nil {
				return nil, err
			}
		case *StopMonitor:
		default:
			return nil, ErrUnknownMessage
	}
	
	return json.Marshal(&obj)
}

func Decode(data []byte) (Message, error) {
	var wrapper messageWrapper
	if err := json.Unmarshal(data,&wrapper); err != nil {
    		return nil,err
    	}
	
	switch wrapper.Name {
		case StartMonitorName:
			var cmd StartMonitor
			if err := json.Unmarshal(wrapper.Data,&cmd); err != nil {
				return nil, err
			}
			
			return &cmd, nil
		case StopMonitorName:
			return &StopMonitor{}, nil
		default:
			return nil, ErrUnknownMessage
	}
	
	return nil,nil
}
