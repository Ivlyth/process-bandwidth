package engine

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/pkg/sync"
	"github.com/pkg/errors"
	psnet "github.com/shirou/gopsutil/v3/net"
	"net"
)

var interfacesMap = &sync.Map[string, *Interface]{}

func initInterfaces() error {
	nics, err := psnet.Interfaces()
	if err != nil {
		return errors.Wrap(err, "get interfaces failed")
	}

	for _, nic := range nics {
		iface := &Interface{
			Name: nic.Name,
		}
		iface.Init(func() {})

		for _, addr := range nic.Addrs {
			ip, _, err := net.ParseCIDR(addr.Addr)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("parse addr for iface %s failed", nic.Name))
			}
			interfacesMap.Store(ip.String(), iface)
		}
	}
	return nil
}

type Interface struct {
	Name string
	RWEventCounter
}
