package command

import (
	"encoding/json"
	"errors"

	"github.com/docker/cli/cli/context/store"
)

// DockerContext is a typed representation of what we put in Context metadata
type DockerContext struct {
	Description      string
	AdditionalFields map[string]interface{}
}

// MarshalJSON implements custom JSON marshalling
func (dc DockerContext) MarshalJSON() ([]byte, error) {
	s := map[string]interface{}{}
	if dc.Description != "" {
		s["Description"] = dc.Description
	}
	if dc.AdditionalFields != nil {
		for k, v := range dc.AdditionalFields {
			s[k] = v
		}
	}
	return json.Marshal(s)
}

// UnmarshalJSON implements custom JSON marshalling
func (dc *DockerContext) UnmarshalJSON(payload []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}
	for k, v := range data {
		switch k {
		case "Description":
			dc.Description = v.(string)
		default:
			if dc.AdditionalFields == nil {
				dc.AdditionalFields = make(map[string]interface{})
			}
			dc.AdditionalFields[k] = v
		}
	}
	return nil
}

// GetDockerContext extracts metadata from stored context metadata
func GetDockerContext(storeMetadata store.Metadata) (DockerContext, error) {
	if storeMetadata.Metadata == nil {
		// can happen if we save endpoints before assigning a context metadata
		// it is totally valid, and we should return a default initialized value
		return DockerContext{}, nil
	}
	res, ok := storeMetadata.Metadata.(DockerContext)
	if !ok {
		return DockerContext{}, errors.New("context metadata is not a valid DockerContext")
	}
	return res, nil
}
