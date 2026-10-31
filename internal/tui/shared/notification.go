package shared

type Level uint8

const (
	Info Level = iota
	Success
	Warning
	Error
)
