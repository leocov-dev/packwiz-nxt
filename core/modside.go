package core

type ModSide string

// The four possible values of Side (the side that the mod is on) are "server", "client", "both", and "" (equivalent to "both")
const (
	ServerSide    ModSide = "server"
	ClientSide    ModSide = "client"
	UniversalSide ModSide = "both"
	EmptySide     ModSide = ""
)
