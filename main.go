package main

import (
	"maunium.net/go/mautrix/bridgev2/matrix/mxmain"
)

var (
	Tag       = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Create connector without config - mxmain will handle config loading
	connector := &SnapchatConnector{}

	// Initialize bridge main
	m := mxmain.BridgeMain{
		Name:        "mautrix-snapchat",
		Description: "A Matrix-Snapchat bridge",
		URL:         "https://github.com/beeper/snapchat-bridge",
		Version:     "1.0.0",
		Connector:   connector,
	}

	m.InitVersion(Tag, Commit, BuildTime)
	m.Run()
}