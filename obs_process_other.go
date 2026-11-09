//go:build !windows

package main

import "errors"

// Unsupported platforms retain normal retries rather than assuming OBS is absent.
func localOBSRunning() (bool, error) { return false, errors.New("OBS process detection unsupported") }
