// Package common provides shared transport helpers for the CPA plugin.
package common

import "encoding/json"

// HostCall is the bridge used by services to invoke CPA host callbacks.
type HostCall func(method string, payload any) (json.RawMessage, error)
