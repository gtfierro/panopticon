package main

import (
	"fmt"
	"github.com/gtfierro/easyssh"
	"github.com/pkg/errors"
	"os"
)

type ProcessMonitor struct {
	programs []Program
	ssh      *easyssh.MakeConfig
}

func NewProcessMonitor(cfg SSHConfig) *ProcessMonitor {
	// if key is specified, check that the path exists
	if len(cfg.Key) > 0 && len(cfg.Password) == 0 {
		if _, err := os.Stat(cfg.Key); os.IsNotExist(err) {
			log.Fatal(errors.Wrap(err, "Key file for SSH does not exist"))
		}
		cfg.Password = ""
	} else if len(cfg.Password) == 0 {
		log.Fatal("Must provide either a password or keyfile for SSH")
		cfg.Key = ""
	}

	if len(cfg.Port) == 0 {
		cfg.Port = "22"
	}

	if len(cfg.Server) == 0 {
		log.Fatal("Must specify an SSH server")
	}

	if len(cfg.User) == 0 {
		log.Fatal("Must specify an SSH user")
	}

	pm := &ProcessMonitor{
		programs: []Program{},
		ssh: &easyssh.MakeConfig{
			User:   cfg.User,
			Server: cfg.Server,
			Key:    cfg.Key,
			Port:   cfg.Port,
		},
	}
	log.Infof("SSH: %+v", pm.ssh)
	return pm
}

func (pm *ProcessMonitor) addProgram(p Program) {
	pm.programs = append(pm.programs, p)
}

func (pm *ProcessMonitor) Run() chan Failure {
	failures := make(chan Failure)
	go func() {
		for _, program := range pm.programs {
			log.Info(printYellow("Checking ", program.Name, " on host ", pm.ssh.Server))
			response, err := pm.ssh.Run(fmt.Sprintf("pgrep %s", program.Process))
			if err != nil {
				failures <- failedSSHContext{Name: program.Name, Process: program.Process, Host: pm.ssh.Server, Error: err}
				break
			}
			if len(response) == 0 {
				failures <- failedProgramContext{Name: program.Name, Process: program.Process, Host: pm.ssh.Server, Error: err}
			}
		}
		close(failures)
	}()
	return failures
}
