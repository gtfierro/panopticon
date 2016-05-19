package main

import (
	"bytes"
)

type Failure interface {
	Message() string
}

var failedHostTemplate = `
Host "{{.Name}}" with address {{.Host}} has failed to respond to a ping. 

Error: 
{{.Error}}

such templating wow
Gabe
`

type failedHostContext struct {
	Host  string
	Name  string
	Error error
}

func (ctx failedHostContext) Message() string {
	var buf bytes.Buffer
	FailedHost.Execute(&buf, ctx)
	return buf.String()
}

func failHost(h failedHostContext) string {
	var buf bytes.Buffer
	FailedHost.Execute(&buf, h)
	return buf.String()
}

var failedProgramTemplate = `
Program "{{.Name}}" (process {{.Process}}) has failed on host {{.Host}}. We could not detect
any PID using "pgrep {{.Process}}".

Error:
{{.Error}}

{{if ne .LogOutput ""}}
{{.LogOutput}}
{{else}}
Configuration for {{.Name}} did not specify a log to read from.
{{end}}
`

type failedProgramContext struct {
	Name      string
	Process   string
	Host      string
	Error     error
	LogOutput string
}

func (ctx failedProgramContext) Message() string {
	var buf bytes.Buffer
	FailedProgram.Execute(&buf, ctx)
	return buf.String()
}

var failedSSHTemplate = `
Could not log into Host {{.Host}} to verify whether program {{.Program}}({{.Process}}) is running.

Error:
{{.Error}}
`

type failedSSHContext struct {
	Name    string
	Process string
	Host    string
	Error   error
}

func (ctx failedSSHContext) Message() string {
	var buf bytes.Buffer
	FailedSSH.Execute(&buf, ctx)
	return buf.String()
}
