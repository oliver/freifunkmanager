package ssh

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	assert := assert.New(t)

	mgmt := NewManager("~/.ssh/id_rsa")
	assert.NotNil(mgmt, "no new manager created")

	mgmt.ConnectTo(net.TCPAddr{IP: net.ParseIP("2a06:8782:ffbb:1337::127"), Port: 22})

	mgmt.RunEverywhere("echo 13", SSHResultToStringHandler(func(result string, err error) {
		assert.NoError(err)

		assert.Equal("13", result)
	}))
	result, err := mgmt.RunOn(net.TCPAddr{IP: net.ParseIP("2a06:8782:ffbb:1337::127"), Port: 22}, "echo 16")
	assert.NoError(err)

	str := SSHResultToString(result)
	resultInt, _ := strconv.Atoi(str)

	assert.Equal(16, resultInt)

	mgmt.Close()
}