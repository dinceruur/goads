package goads

// TransMode ADS transfer mode
type TransMode uint32

const (
	NoTrans TransMode = iota
	TransClientCycle
	TransClientOnCha
	TransServerCycle
	TransServerOnCha
)

const (
	StateInvalid uint16 = iota
	StateIdle
	StateReset
	StateInit
	StateStart
	StateRun
	StateStop
	StateSaveConfig
	StateLoadConfig
	StatePowerFailure
	StatePowerGood
	StateError
	StateShutdown
	StateSuspend
	StateResume
	StateConfig
	StateReConfig
	StateStopping
	StateIncompatible
	StateException
)

const (
	PortSystemService uint16 = 10000
	PortPLC1          uint16 = 851
)
