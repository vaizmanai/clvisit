package processor

import (
	"clvisit/common"
	log "github.com/sirupsen/logrus"
	"github.com/tatsushid/go-fastping"
	"net"
	"time"
)

const (
	WaitRefreshAgents = 180
)

var (
	agents []Agent
)

type Agent struct {
	Metric  int
	Address string
}

func sortAgents() {
	for i := 0; i < len(agents); i++ {
		for j := i; j < len(agents); j++ {
			if agents[i].Metric > agents[j].Metric && agents[j].Metric != -1 {
				tmp := agents[i]
				agents[i] = agents[j]
				agents[j] = tmp
			}
		}
	}
	printAgentsMetric()
}

func updateAgentsMetric() {
	for i := 0; i < len(agents); i++ {
		agents[i].Metric = UpdateAgentMetric(agents[i].Address)
	}
	log.Infof("обновили метрики агентов")
}

func UpdateAgentMetric(address string) int {
	metric := -1
	p := fastping.NewPinger()

	ra, err := net.ResolveIPAddr("ip4:icmp", address)
	if err != nil {
		return metric
	}

	p.AddIPAddr(ra)
	p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		metric = int(rtt.Nanoseconds() / 1000)
	}
	_ = p.Run()
	return metric
}

func printAgentsMetric() {
	for i := 0; i < len(agents); i++ {
		log.Debugf("метрика для %s - %s", agents[i].Address, agents[i].Metric)
	}
}

func refreshAgents() {
	if common.Flags.ChRefreshAgents == nil {
		common.Flags.ChRefreshAgents = make(chan bool)
	}

	for {
		updateAgentsMetric()
		sortAgents()

		select {
		case <-time.After(time.Duration(WaitRefreshAgents) * time.Second):
		case <-common.Flags.ChRefreshAgents:
		}
	}
}
