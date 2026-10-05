package ui

type SyncState string

const (
	SyncStateHidden  SyncState = ""
	SyncStateRunning SyncState = "running"
	SyncStateOK      SyncState = "ok"
	SyncStateWarn    SyncState = "warn"
	SyncStateError   SyncState = "error"
)

type SyncStatus struct {
	State   SyncState
	Message string
}

type Options struct {
	FullScreen       bool
	Height           int
	Layout           string
	ShowAddress      bool
	InitialSync      SyncStatus
	SyncStatusStream <-chan SyncStatus
}
