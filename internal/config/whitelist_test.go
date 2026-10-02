package config

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhitelistBareIPv4(t *testing.T) {
	nets, err := createIPNets([]string{"198.51.100.7"})
	assert.NoError(t, err)
	assert.Equal(t, "198.51.100.7/32", nets[0].String())
	assert.False(t, nets[0].Contains(net.ParseIP("198.51.100.8")))
}

func TestWhitelistBareIPv6(t *testing.T) {
	nets, err := createIPNets([]string{"2001:db8::1"})
	assert.NoError(t, err)
	assert.Equal(t, "2001:db8::1/128", nets[0].String())
	assert.True(t, nets[0].Contains(net.ParseIP("2001:db8::1")))
	assert.False(t, nets[0].Contains(net.ParseIP("2001:db8::2")))
	assert.False(t,
		nets[0].Contains(net.ParseIP("2001:db8:ffff::9")))
}

func TestWhitelistBareIPv4Mapped(t *testing.T) {
	nets, err := createIPNets([]string{"::ffff:198.51.100.7"})
	assert.NoError(t, err)
	assert.True(t, nets[0].Contains(net.ParseIP("198.51.100.7")))
	assert.False(t, nets[0].Contains(net.ParseIP("198.51.100.8")))
}

func TestWhitelistCIDRUnchanged(t *testing.T) {
	nets, err := createIPNets(
		[]string{"10.0.0.0/8", "2001:db8::/48"})
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.0/8", nets[0].String())
	assert.Equal(t, "2001:db8::/48", nets[1].String())
}

func TestWhitelistInvalid(t *testing.T) {
	for _, bad := range []string{"", "nope", "1.2.3.4/33",
		"2001:db8::/129", "1.2.3", "2001:db8:::1"} {
		_, err := createIPNets([]string{bad})
		assert.Error(t, err, bad)
	}
}
