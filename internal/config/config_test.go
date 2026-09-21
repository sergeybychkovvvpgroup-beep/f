package config

import "testing"

func TestDefaultShowsSSHListOnStart(t *testing.T) {
	if !DefaultFile().ShowListOnStart {
		t.Fatal("default picker must show the SSH list so tabs and preview are visible immediately")
	}
}
