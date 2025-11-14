package lb

func NewBackendServers(serverCount int) *BackendServers {
	cnt := serverCount
	if cnt < 0 {
		cnt = 0
	}
	return &BackendServers{
		servers: make([]BackendServer, 0, cnt),
	}
}

func (bs *BackendServers) Append(server BackendServer) {
	if bs == nil {
		return
	}
	bs.servers = append(bs.servers, server)
}

func (bs *BackendServers) GetBackendServers() []BackendServer {
	if bs == nil {
		return nil
	}
	return bs.servers
}

func (bs *BackendServers) GetMatchedBackendServerCount() int {
	if bs == nil {
		return 0
	}
	return bs.matchedRunningBackendServerCount
}

func (bs *BackendServers) SetMatchedBackendServerCount(cnt int) {
	if bs == nil || cnt < 0 {
		return
	}
	bs.matchedRunningBackendServerCount = cnt
}

func (bs *BackendServers) GetWithIPAddressBackendServerCount() int {
	if bs == nil {
		return 0
	}
	return bs.withAddressBackendServerCount
}

func (bs *BackendServers) SetWithAddressBackendServerCount(cnt int) {
	if bs == nil || cnt < 0 {
		return
	}
	bs.withAddressBackendServerCount = cnt
}
