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
}

type Manager struct {
	SM     *ServerMonitor
	mailer gomail.SendCloser
	config Config
	hosts  map[string]Host
}

func NewManager() *Manager {
	return &Manager{
		SM:    NewServerMonitor(10),
		hosts: make(map[string]Host),
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

type Config struct {
	Mail  Mailconfig
	Loop  string // time.Duration
	Hosts []Host
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

	//m.mailRecipients = mailConfig.Key("recipients").Strings(",")

	log.Infof("Email server: %+v", m.config.Mail)
	mailer := gomail.NewDialer(m.config.Mail.Server, m.config.Mail.Port, m.config.Mail.Username, m.config.Mail.Password)
	m.mailer, err = mailer.Dial()
	if err != nil {
		log.Critical(errors.Wrap(err, "Could not dial email server"))
	}

	for _, host := range m.config.Hosts {
		m.SM.addDestination(host)
		m.hosts[host.Host] = host
	}
}

func (m *Manager) sendMail(text string) {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.config.Mail.Username)
	msg.SetHeader("To", m.config.Mail.Recipients...)
	msg.SetHeader("Subject", "Chair Report")
	msg.SetBody("text/plain", text)

	if err := m.mailer.Send(m.config.Mail.Username, m.config.Mail.Recipients, msg); err != nil {
		log.Critical(errors.Wrap(err, "Could not send mail"))
	}
}

func (m *Manager) Run() {
	dur, err := time.ParseDuration(m.config.Loop)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Could not parse duration from config"))
	}
	// run once first
	for ctx := range m.SM.Run() {
		if ctx.Error != nil {
			log.Errorf("%+v", ctx)
			m.sendMail(failHost(ctx))
		}
	}
	// then start ticker
	ticker := time.Tick(dur)
	for _ = range ticker {
		log.Notice("-----Starting Pings-----")
		for ctx := range m.SM.Run() {
			if ctx.Error != nil {
				log.Errorf("%+v", ctx)
				m.sendMail(failHost(ctx))
			}
		}
		log.Notice("-----Finished!-----")
	}
}

func main() {
	m := NewManager()
	m.LoadConfig(os.Args[1])
	m.Run()
}
