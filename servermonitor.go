package main

// sysctl -w net.ipv4.ping_group_range="0 0"

import (
	"github.com/pkg/errors"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"net"
	"os"
	"sync"
	"time"
)

type pingAddress struct {
	IPAddr *net.IPAddr
	Addr   string
	Is4    bool
}

func newPingAddress(address string) *pingAddress {
	addr, err := net.ResolveIPAddr("ip", address)
	if err != nil {
		log.Critical(errors.Wrap(err, "Could not resolve IP addr"))
	}
	log.Infof("Got addr %s, zone %s", addr.IP, addr.Zone)
	return &pingAddress{
		IPAddr: addr,
		Addr:   address,
		Is4:    addr.IP.To4() != nil,
	}
}

type ServerMonitor struct {
	addresses map[string]*pingAddress
	wait      map[string]chan error
	waitL     sync.RWMutex
	Timeout   int
	icmp4     *icmp.PacketConn
	icmp4L    sync.RWMutex
	icmp6     *icmp.PacketConn
	icmp6L    sync.RWMutex
	errors    chan error
	hosts     map[string]Host
}

func NewServerMonitor(timeout int) *ServerMonitor {
	var err error
	sm := &ServerMonitor{
		Timeout:   timeout,
		addresses: make(map[string]*pingAddress),
		wait:      make(map[string]chan error),
		errors:    make(chan error),
		hosts:     make(map[string]Host),
	}

	sm.icmp6, err = icmp.ListenPacket("udp6", "::%he-ipv6")
	if err != nil {
		log.Fatal(errors.Wrap(err, "Could not listen ipv6 icmp socket"))
	}

	go func() {
		buf := make([]byte, 1500)
		for {
			n, peer, err := sm.icmp6.ReadFrom(buf)
			if err != nil {
				sm.listenICMP6()
				sm.errors <- errors.Wrap(err, "Could not read to listen for ping")
				continue
			}
			rm, err := icmp.ParseMessage(58, buf[:n])
			if err != nil {
				sm.listenICMP6()
				sm.errors <- errors.Wrap(err, "Could not parse ping message")
				continue
			}
			switch rm.Type {
			case ipv6.ICMPTypeEchoReply:
				address := peer.(*net.UDPAddr).IP.String()
				log.Info(printGreen("Got ipv6 ping from ", address))
				sm.waitL.RLock()
				c := sm.wait[address]
				sm.waitL.RUnlock()
				c <- nil
			default:
				log.Infof("Got %+v", rm)
			}
			sm.listenICMP6()
		}
	}()

	sm.icmp4, err = icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		log.Fatal(errors.Wrap(err, "Could not listen ipv4 icmp socket"))
	}

	go func() {
		buf := make([]byte, 1500)
		for {
			n, peer, err := sm.icmp4.ReadFrom(buf)
			if err != nil {
				sm.listenICMP4()
				sm.errors <- errors.Wrap(err, "Could not read to listen for ping")
				continue
			}
			rm, err := icmp.ParseMessage(1, buf[:n])
			if err != nil {
				sm.listenICMP4()
				sm.errors <- errors.Wrap(err, "Could not parse ping message")
				continue
			}
			switch rm.Type {
			case ipv4.ICMPTypeEchoReply:
				address := peer.(*net.UDPAddr).IP.String()
				log.Info(printGreen("Got ipv4 ping from ", address))
				sm.waitL.RLock()
				c := sm.wait[address]
				sm.waitL.RUnlock()
				c <- nil
			default:
				log.Infof("Got %+v", rm)
			}
			sm.listenICMP4()
		}
	}()

	return sm
}

func (sm *ServerMonitor) listenICMP6() {
	var err error
	sm.icmp6L.Lock()
	defer func() {
		sm.icmp6L.Unlock()
	}()
	sm.icmp6.Close()
	if sm.icmp6, err = icmp.ListenPacket("udp6", "::%he-ipv6"); err != nil {
		log.Fatal(errors.Wrap(err, "Could not listen ipv6 icmp socket"))
	}
}
func (sm *ServerMonitor) listenICMP4() {
	var err error
	sm.icmp4L.Lock()
	defer sm.icmp4L.Unlock()
	sm.icmp4.Close()
	if sm.icmp4, err = icmp.ListenPacket("udp4", "0.0.0.0"); err != nil {
		log.Fatal(errors.Wrap(err, "Could not listen ipv4 icmp socket"))
	}
}

func (sm *ServerMonitor) addDestination(h Host) {
	sm.addresses[h.Host] = newPingAddress(h.Host)
	sm.hosts[h.Host] = h
}

func (sm *ServerMonitor) ping(address string) error {
	var (
		//c   *icmp.PacketConn
		wm  icmp.Message
		err error
	)
	pingme := sm.addresses[address]
	sm.waitL.Lock()
	sm.wait[pingme.IPAddr.IP.String()] = make(chan error)
	sm.waitL.Unlock()

	wm = icmp.Message{
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte("ping message"),
		},
	}
	if pingme.Is4 {
		wm.Type = ipv4.ICMPTypeEcho
	} else {
		wm.Type = ipv6.ICMPTypeEchoRequest
	}

	wb, err := wm.Marshal(nil)
	if err != nil {
		return errors.Wrap(err, "Could not marshal ping msg")
	}

	if pingme.Is4 {
		sm.icmp4L.Lock()
		sm.icmp4.SetReadDeadline(time.Now().Add(time.Duration(sm.Timeout) * time.Second))
		if _, err := sm.icmp4.WriteTo(wb, &net.UDPAddr{IP: pingme.IPAddr.IP}); err != nil {
			sm.icmp4L.Unlock()
			return errors.Wrap(err, "Could not send ping")
		}
		sm.icmp4L.Unlock()
	} else {
		sm.icmp6L.Lock()
		sm.icmp6.SetReadDeadline(time.Now().Add(time.Duration(sm.Timeout) * time.Second))
		if _, err := sm.icmp6.WriteTo(wb, &net.UDPAddr{IP: pingme.IPAddr.IP, Zone: "he-ipv6"}); err != nil {
			sm.icmp6L.Unlock()
			return errors.Wrap(err, "Could not send ping")
		}
		sm.icmp6L.Unlock()
	}

	sm.waitL.RLock()
	c := sm.wait[pingme.IPAddr.IP.String()]
	sm.waitL.RUnlock()
	select {
	case err = <-sm.errors:
	case err = <-c:
	}

	return err
}

func (sm *ServerMonitor) Run() chan failedHostContext {
	failures := make(chan failedHostContext)
	go func() {
		for address, _ := range sm.addresses {
			log.Info(printYellow("Pinging ", address))
			h := sm.hosts[address]
			if err := sm.ping(address); err != nil {
				failures <- failedHostContext{Host: h.Host, Name: h.Name, Error: err}
			}
		}
		close(failures)
	}()
	return failures
}
