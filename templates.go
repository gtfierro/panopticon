package main

import (
	"bytes"
)

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

func failHost(h failedHostContext) string {
	var buf bytes.Buffer
	FailedHost.Execute(&buf, h)
	return buf.String()
}
