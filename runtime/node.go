package runtime

import (
	"bytes"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"

	yanicData "github.com/FreifunkBremen/yanic/data"
	"github.com/FreifunkBremen/yanic/lib/jsontime"
	yanicRuntime "github.com/FreifunkBremen/yanic/runtime"

	"github.com/FreifunkBremen/freifunkmanager/ssh"
)

const (
	SSHUpdateHostname   = "uci set system.@system[0].hostname='%s'; uci set wireless.priv_radio0.ssid=\"offline-$(uci get system.@system[0].hostname)\"; uci set wireless.priv_radio1.ssid=\"offline-$(uci get system.@system[0].hostname)\"; uci commit; echo $(uci get system.@system[0].hostname) > /proc/sys/kernel/hostname; wifi"
	SSHUpdateOwner      = "uci set gluon-node-info.@owner[0].contact='%s';uci commit gluon-node-info;"
	SSHUpdateLocation   = "uci set gluon-node-info.@location[0].latitude='%f';uci set gluon-node-info.@location[0].longitude='%f';uci set gluon-node-info.@location[0].share_location=1;uci commit gluon-node-info;"
	SSHUpdateWifiFreq24 = "if [ \"$(uci get wireless.radio0.hwmode | grep -c g)\" -ne 0 ]; then uci set wireless.radio0.channel='%d'; uci set wireless.radio0.txpower='%d'; elif [ \"$(uci get wireless.radio1.hwmode | grep -c g)\" -ne 0 ]; then uci set wireless.radio1.channel='%d'; uci set wireless.radio1.txpower='%d'; fi;"
	SSHUpdateWifiFreq5  = "if [ \"$(uci get wireless.radio0.hwmode | grep -c a)\" -ne 0 ]; then uci set wireless.radio0.channel='%d'; uci set wireless.radio0.txpower='%d'; elif [ \"$(uci get wireless.radio1.hwmode | grep -c a)\" -ne 0 ]; then uci set wireless.radio1.channel='%d'; uci set wireless.radio1.txpower='%d'; fi;"
)

type Node struct {
	Lastseen jsontime.Time      `json:"lastseen"`
	NodeID   string             `json:"node_id"`
	Hostname string             `json:"hostname"`
	Location yanicData.Location `json:"location"`
	Wireless yanicData.Wireless `json:"wireless"`
	Owner    string             `json:"owner"`
	Address  net.IP             `json:"-"`
	Stats    struct {
		Wireless yanicData.WirelessStatistics `json:"wireless"`
		Clients  yanicData.Clients            `json:"clients"`
	} `json:"statistics"`
}

func NewNode(nodeOrigin *yanicRuntime.Node) *Node {
	if nodeinfo := nodeOrigin.Nodeinfo; nodeinfo != nil {
		node := &Node{
			Hostname: nodeinfo.Hostname,
			NodeID:   nodeinfo.NodeID,
		}
		for _, ip := range nodeinfo.Network.Addresses {
			ipAddr := net.ParseIP(ip)
			if node.Address == nil || ipAddr.IsGlobalUnicast() {
				node.Address = ipAddr
			}
		}
		if owner := nodeinfo.Owner; owner != nil {
			node.Owner = owner.Contact
		}
		if location := nodeinfo.Location; location != nil {
			node.Location = *location
		}
		if wireless := nodeinfo.Wireless; wireless != nil {
			node.Wireless = *wireless
		}
		if stats := nodeOrigin.Statistics; stats != nil {
			node.Stats.Clients = stats.Clients
			node.Stats.Wireless = stats.Wireless
		}
		return node
	}
	return nil
}

func (n *Node) SSHUpdate(ssh *ssh.Manager, iface string, oldnode *Node) {
	addr := n.GetAddress(iface)

	if oldnode == nil || n.Hostname != oldnode.Hostname {
		ssh.ExecuteOn(addr, fmt.Sprintf(SSHUpdateHostname, n.Hostname))
	}
	if oldnode == nil || n.Owner != oldnode.Owner {
		ssh.ExecuteOn(addr, fmt.Sprintf(SSHUpdateOwner, n.Owner))
	}
	if oldnode == nil || !locationEqual(n.Location, oldnode.Location) {
		ssh.ExecuteOn(addr, fmt.Sprintf(SSHUpdateLocation, n.Location.Latitude, n.Location.Longitude))
	}
	if oldnode == nil || !wirelessEqual(n.Wireless, oldnode.Wireless) {
		ssh.ExecuteOn(addr, fmt.Sprintf(SSHUpdateWifiFreq24, n.Wireless.Channel24, n.Wireless.TxPower24, n.Wireless.Channel24, n.Wireless.TxPower24))
		ssh.ExecuteOn(addr, fmt.Sprintf(SSHUpdateWifiFreq5, n.Wireless.Channel5, n.Wireless.TxPower5, n.Wireless.Channel5, n.Wireless.TxPower5))
		ssh.ExecuteOn(addr, "wifi")
		// send warning for running wifi, because it kicks clients from node
		log.Warn("[cmd] wifi ", n.NodeID)
	}
	oldnode = n
}
func (n *Node) GetAddress(iface string) net.TCPAddr {
	return net.TCPAddr{IP: n.Address, Port: 22, Zone: iface}
}
func (n *Node) Update(node *yanicRuntime.Node) {
	if nodeinfo := node.Nodeinfo; nodeinfo != nil {
		n.Hostname = nodeinfo.Hostname
		n.NodeID = nodeinfo.NodeID
		n.Address = node.Address.IP

		if owner := nodeinfo.Owner; owner != nil {
			n.Owner = owner.Contact
		}
		if location := nodeinfo.Location; location != nil {
			n.Location = *location
		}
		if wireless := nodeinfo.Wireless; wireless != nil {
			n.Wireless = *wireless
		}
	}
}
func (n *Node) IsEqual(node *Node) bool {
	if n.NodeID != node.NodeID {
		return false
	}
	if !bytes.Equal(n.Address, node.Address) {
		return false
	}
	if n.Hostname != node.Hostname {
		return false
	}
	if n.Owner != node.Owner {
		return false
	}
	if !locationEqual(n.Location, node.Location) {
		return false
	}
	if !wirelessEqual(n.Wireless, node.Wireless) {
		return false
	}
	return true
}

func locationEqual(a, b yanicData.Location) bool {
	if a.Latitude != b.Latitude {
		return false
	}
	if a.Longitude != b.Longitude {
		return false
	}
	if a.Altitude != b.Altitude {
		return false
	}
	return true
}

func wirelessEqual(a, b yanicData.Wireless) bool {
	if a.Channel24 != b.Channel24 {
		return false
	}
	if a.Channel5 != b.Channel5 {
		return false
	}
	if a.TxPower24 != b.TxPower24 {
		return false
	}
	if a.TxPower5 != b.TxPower5 {
		return false
	}
	return true
}
