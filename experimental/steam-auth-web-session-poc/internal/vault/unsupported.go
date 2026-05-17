//go:build !windows

package vault

import "errors"

func New() (Vault, error) {
	return nil, errors.New("secure token storage is only implemented with Windows DPAPI in this POC")
}
