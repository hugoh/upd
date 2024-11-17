package internal

const (
	AppName  = "upd"
	AppShort = "Tool to monitor if the network connection is up."
	AppDesc  = `
	Runs HTTP, TCP or DNS checks on a regular basis.
    If all checks fail, runs down action on a regular basis until the
    connection is back up.
	`
)
