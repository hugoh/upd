package internal

import "fmt"

const (
	AppName  = "upd"
	AppShort = "Tool to monitor if the network connection is up."
)

func Version(version string) string {
	return fmt.Sprintf("%s version %s", AppName, version)
}
