package main

import (
	"github.com/fatih/color"
	"github.com/go-yaml/yaml"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"gopkg.in/gomail.v2"
	"io/ioutil"
	"os"
	"text/template"
	"time"
)

var log = logging.MustGetLogger("panopticon")
var format = "%{color}%{level} %{time:Jan 02 15:04:05} %{shortfile}%{color:reset} ▶ %{message}"
var FailedHost *template.Template
var FailedProgram *template.Template
var FailedSSH *template.Template

var printYellow = color.New(color.FgYellow).SprintFunc()
var printGreen = color.New(color.FgGreen).SprintFunc()

func init() {
	var logBackend = logging.NewLogBackend(os.Stderr, "", 0)
	logBackendLeveled := logging.AddModuleLevel(logBackend)
	logging.SetBackend(logBackendLeveled)
	logging.SetFormatter(logging.MustStringFormatter(format))

	var err error
	FailedHost, err = template.New("failedhost").Parse(failedHostTemplate)
	if err != nil {
		log.Fatal(err)
	}

	FailedProgram, err = template.New("failedprogram").Parse(failedProgramTemplate)
	if err != nil {
		log.Fatal(err)
	}

	FailedSSH, err = template.New("failedSSH").Parse(failedSSHTemplate)
	if err != nil {
		log.Fatal(err)
	}
}

type Manager struct {
	SM       *ServerMonitor
	mailer   *gomail.Dialer
	config   Config
	hosts    map[string]Host
	monitors []*ProcessMonitor
}

func NewManager() *Manager {
	return &Manager{
		hosts:    make(map[string]Host),
		monitors: []*ProcessMonitor{},
	}
}

type Mailconfig struct {
	Server     string
	Port       int
	Username   string
	Password   string
	Recipients []string
}

type Host struct {
	Host string
	Name string
}

type Program struct {
	Name        string
	Process     string
	LogLocation string
}

type SSHConfig struct {
	User     string
	Server   string
	Password string
	Key      string
	Port     string
}

type ProgramConfig struct {
	Server   SSHConfig
	Programs []Program
}

type Config struct {
	Mail     Mailconfig
	Loop     string // time.Duration
	Hosts    []Host
	Monitors []ProgramConfig
}

func (m *Manager) LoadConfig(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Could not open config file"))
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Could not read config file"))
	}

	m.config = Config{}

	err = yaml.Unmarshal(b, &m.config)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Could not load config"))
	}

	log.Infof("Email server: %+v", m.config.Mail)
	m.mailer = gomail.NewDialer(m.config.Mail.Server, m.config.Mail.Port, m.config.Mail.Username, m.config.Mail.Password)

	if len(m.config.Hosts) > 0 {
		m.SM = NewServerMonitor(10)
	}
	for _, host := range m.config.Hosts {
		m.SM.addDestination(host)
		m.hosts[host.Host] = host
	}

	for _, monitor := range m.config.Monitors {
		log.Infof("SSH server: %+v", monitor.Server)
		pm := NewProcessMonitor(monitor.Server)
		for _, program := range monitor.Programs {
			pm.addProgram(program)
		}
		m.monitors = append(m.monitors, pm)
	}
}

func (m *Manager) sendMail(text string) {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.config.Mail.Username)
	msg.SetHeader("To", m.config.Mail.Recipients...)
	msg.SetHeader("Subject", "Chair Report")
	msg.SetBody("text/plain", text)

	if err := m.mailer.DialAndSend(msg); err != nil {
		log.Critical(errors.Wrap(err, "Could not send mail"))
	}
}

func (m *Manager) run() {
	if m.SM != nil {
		log.Notice("-----Starting Pings-----")
		for ctx := range m.SM.Run() {
			log.Errorf("%+v", ctx)
			m.sendMail(ctx.Message())
		}
		log.Notice("-----Finished!-----")
	}
	log.Notice("-----Starting Process Monitors-----")
	for _, monitor := range m.monitors {
		for ctx := range monitor.Run() {
			log.Errorf("%T %+v", ctx, ctx)
			m.sendMail(ctx.Message())
		}
	}
	log.Notice("-----Finished!-----")
}

func (m *Manager) Start() {
	dur, err := time.ParseDuration(m.config.Loop)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Could not parse duration from config"))
	}
	// run once
	m.run()
	// then start ticker
	ticker := time.Tick(dur)
	for _ = range ticker {
		m.run()
	}
}

func main() {
	m := NewManager()
	m.LoadConfig(os.Args[1])
	m.Start()
}
