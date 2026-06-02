package ping

// Target describes a single probe destination.
type Target struct {
	HostName string
	Protocol string
	TCPPort  int
}

func (t Target) WithDefaults(defaultProtocol string, defaultPort int) Target {
	if t.Protocol == "" {
		t.Protocol = defaultProtocol
	}
	if t.TCPPort == 0 {
		t.TCPPort = defaultPort
	}
	return t
}
