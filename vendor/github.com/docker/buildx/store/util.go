package store

import (
	"os"
	"strings"

	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/pkg/errors"
)

func ValidateName(s string) (string, error) {
	if !namePattern.MatchString(s) {
		return "", errors.Errorf("invalid name %s, name needs to start with a letter and may not contain symbols, except ._-", s)
	}
	return strings.ToLower(s), nil
}

func GenerateName(txn *Txn) (string, error) {
	var name string
	for i := 0; i < 6; i++ {
		name = namesgenerator.GetRandomName(i)
		if _, err := txn.NodeGroupByName(name); err != nil {
			if !os.IsNotExist(errors.Cause(err)) {
				return "", err
			}
		} else {
			continue
		}
		return name, nil
	}
	return "", errors.Errorf("failed to generate random name")
}
