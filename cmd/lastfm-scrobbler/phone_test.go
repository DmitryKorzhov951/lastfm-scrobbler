package main

import (
	"net"
	"reflect"
	"testing"
)

func TestАдресКомпьютераДляТелефона(t *testing.T) {
	ips := func(list ...string) []net.IP {
		out := make([]net.IP, 0, len(list))
		for _, s := range list {
			out = append(out, net.ParseIP(s))
		}
		return out
	}
	got := orderIPs(net.ParseIP("192.168.0.105"), ips(
		"127.0.0.1", "172.24.80.1", "10.8.0.2", "192.168.0.105", "169.254.10.20", "8.8.4.4", "fe80::1", "192.168.56.1"))
	want := []string{"192.168.0.105", "192.168.56.1", "10.8.0.2", "172.24.80.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("адреса: %v, ожидалось %v", got, want)
	}

	if got := orderIPs(nil, ips("10.0.0.3", "192.168.1.4")); !reflect.DeepEqual(got, []string{"192.168.1.4", "10.0.0.3"}) {
		t.Errorf("без адреса маршрута сначала домашняя сеть 192.168: %v", got)
	}
	if got := orderIPs(net.ParseIP("203.0.113.5"), ips("203.0.113.5")); len(got) != 0 {
		t.Errorf("внешний адрес не подходит: %v", got)
	}
}
