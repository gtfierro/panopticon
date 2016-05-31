Panopticon
==========

Currently has to be run as root for the ICMP messages.

Look in [`monitor.yaml`](https://github.com/gtfierro/panopticon/blob/master/monitor.yaml) for example config.

Run with `panopticon monitor.yaml`

May need to run
```
$ sudo sysctl -w net.ipv4.ping_group_range="0 0"
```

## Coming Soon

- [x] Remote process monitoring
- [ ] Unpriveleged pings (may be impossible w/ Go?)
- [ ] Reporting logs in process monitoring
