// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package pkg

import (
	"errors"
	"fmt"
)

var (
	EnvVariableNotSetError = errors.New("environment variable is not set")
	EmptyConfigValueError  = errors.New("config value is empty")
)

func NewEnvVariableNotSetError(varName string) error {
	return fmt.Errorf("%w: %s", EnvVariableNotSetError, varName)
}

func NewEmptyConfigValueError(configValueName string) error {
	return fmt.Errorf("%w: %s", EmptyConfigValueError, configValueName)
}
