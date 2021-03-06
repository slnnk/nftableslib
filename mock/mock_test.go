package mock

import (
	"testing"

	"github.com/google/nftables"
	"github.com/sbezverk/nftableslib"
	"golang.org/x/sys/unix"
)

func setActionRedirect(t *testing.T, port int, tproxy bool) *nftableslib.RuleAction {
	ra, err := nftableslib.SetRedirect(port, tproxy)
	if err != nil {
		t.Fatalf("failed to SetRedirect with error: %+v", err)
	}
	return ra
}

func setActionVerdict(t *testing.T, key int, chain ...string) *nftableslib.RuleAction {
	ra, err := nftableslib.SetVerdict(key, chain...)
	if err != nil {
		t.Fatalf("failed to SetVerdict with error: %+v", err)
	}
	return ra
}

func setActionReject(t *testing.T, rejectType, rejectCode int) *nftableslib.RuleAction {
	ra, err := nftableslib.SetReject(rejectType, rejectCode)
	if err != nil {
		t.Fatalf("failed to SetVerdict with error: %+v", err)
	}
	return ra
}

func setIPAddr(t *testing.T, addr string) *nftableslib.IPAddr {
	a, err := nftableslib.NewIPAddr(addr)
	if err != nil {
		t.Fatalf("error %+v return from NewIPAddr for address: %s", err, addr)
	}
	return a
}

func setLog(key int, value []byte) *nftableslib.Log {
	log, _ := nftableslib.SetLog(key, value)
	return log
}

func setSNAT(t *testing.T, attrs *nftableslib.NATAttributes) *nftableslib.RuleAction {
	ra, err := nftableslib.SetSNAT(attrs)
	if err != nil {
		t.Fatalf("error %+v return from SetSNAT call\n", err)
		return nil
	}
	return ra
}
func TestMock(t *testing.T) {
	port1 := 8080
	port2 := 9090
	port3 := 8989
	portRedirect := 15001

	v2ipv4tests := []struct {
		name    string
		rule    nftableslib.Rule
		success bool
	}{
		{
			name: "IPv4 L3 and L4 Source and Destination lists",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Src: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "192.0.2.0"), setIPAddr(t, "192.0.3.0")},
					},
					Dst: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "192.0.12.0"), setIPAddr(t, "192.0.13.0")},
					},
				},
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Src: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1, port2}),
					},
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port3}),
					},
				},
				Log:    setLog(unix.NFTA_LOG_PREFIX, []byte("nftableslib")),
				Action: setActionVerdict(t, unix.NFT_RETURN),
			},
			success: true,
		},
	}
	v2ipv6tests := []struct {
		name    string
		rule    nftableslib.Rule
		success bool
	}{
		{
			name: "IPv4 L3 and L4 Source and Destination lists",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Src: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "fe00:123::1"), setIPAddr(t, "2001:456::")},
					},
					Dst: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "::1234:1"), setIPAddr(t, "::1"), setIPAddr(t, "::")},
					},
				},
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Src: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1, port2}),
					},
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port3}),
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
	}

	ipv4Tests := []struct {
		name    string
		rule    nftableslib.Rule
		success bool
	}{
		{
			name: "L3 redirect proto no TProxy",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Protocol: nftableslib.L3Protocol(unix.IPPROTO_TCP),
				},
				Action: setActionRedirect(t, portRedirect, false),
			},
			success: true,
		},
		{
			name: "L3 redirect proto with TProxy",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Protocol: nftableslib.L3Protocol(unix.IPPROTO_TCP),
				},
				Action: setActionRedirect(t, portRedirect, true),
			},
			success: true,
		},
		{
			name: "Single IPv4 in list, source, no exclusion, with subnet mask",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Src: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "192.16.0.0/16"), setIPAddr(t, "10.16.0.0/16")},
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "Single IPv4 in list, source, no exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Src: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "192.0.2.0")},
					},
				},
				Action: setActionReject(t, unix.NFT_REJECT_ICMPX_UNREACH, unix.NFT_REJECT_ICMPX_HOST_UNREACH),
			},
			success: true,
		},
		{
			name: "Single IPv4 in list, destination, exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Dst: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "192.0.2.1")},
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "Multiple IPv4s in list, source, exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Dst: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "192.0.2.1"), setIPAddr(t, "192.0.3.1"), setIPAddr(t, "192.0.4.1")},
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "Multiple IPv4s in list, destination, no exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Dst: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "192.0.2.1"), setIPAddr(t, "192.0.3.1"), setIPAddr(t, "192.0.4.1")},
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "IPv4 Range, destination, no exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Src: &nftableslib.IPAddrSpec{
						Range: [2]*nftableslib.IPAddr{setIPAddr(t, "1.1.1.0"), setIPAddr(t, "2.2.2.0")},
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "IPv4 Range, destination, no exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Protocol: nftableslib.L3Protocol(unix.IPPROTO_TCP),
				},
				Action: setSNAT(t, &nftableslib.NATAttributes{
					L3Addr: [2]*nftableslib.IPAddr{setIPAddr(t, "5.5.5.5")},
					Port:   [2]uint16{7777},
				})},
			success: true,
		},
	}
	ipv6Tests := []struct {
		name    string
		rule    nftableslib.Rule
		success bool
	}{
		{
			name: "Single IPv6 in list, source, no exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Src: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "2001:0101::1")},
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "Single IPv6 in list, source, no exclusion, CIDR",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Src: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "::1/128")},
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "Single IPv6 in list, source, no exclusion, with subnet mask ",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Src: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "2001:0101::/64")},
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "Single IPv6 in list, destination, exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Dst: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "fe80::1852:15be:a31d:5d2f")},
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "Multiple IPv6s in list, source, exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Dst: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{setIPAddr(t, "2001:0101::1"), setIPAddr(t, "fe80::1852:15be:a31d:5d2f")},
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "Multiple IPv6s in list, destination, no exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Dst: &nftableslib.IPAddrSpec{
						List: []*nftableslib.IPAddr{
							setIPAddr(t, "2001:470:b87e:81::11"),
							setIPAddr(t, "fe80::5054:ff:fe6c:1c4d"),
							setIPAddr(t, "fe80::5054:ff:fecd:2379"),
						},
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake-chain-1"),
			},
			success: true,
		},
		{
			name: "IPv6 Range, destination, no exclusion",
			rule: nftableslib.Rule{
				L3: &nftableslib.L3Rule{
					Dst: &nftableslib.IPAddrSpec{
						Range: [2]*nftableslib.IPAddr{
							setIPAddr(t, "2001:470:b87e:81::11"),
							setIPAddr(t, "2001:470:b87e:89::11"),
						},
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake_chain_1"),
			},
			success: true,
		},
	}

	l4PortTests := []struct {
		name    string
		rule    nftableslib.Rule
		success bool
	}{
		{
			name: "L4 Single source port with verdict",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Src: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1}),
					},
				},
				Action: setActionVerdict(t, unix.NFT_JUMP, "fake_chain_1"),
			},
			success: true,
		},
		{
			name: "L4 Single destination port with verdict",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_UDP,
					Src: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port2}),
					},
				},
				Action: setActionVerdict(t, unix.NFT_RETURN),
			},
			success: true,
		},
		{
			name: "L4 Single destination port with verdict and exclusion",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1}),
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionVerdict(t, unix.NFT_RETURN),
			},
			success: true,
		},
		{
			name: "L4 Single source port with redirect",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Src: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1}),
					},
				},
				Action: setActionRedirect(t, portRedirect, false),
			},
			success: true,
		},
		{
			name: "L4 Single destination port with redirect",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_UDP,
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1}),
					},
				},
				Action: setActionRedirect(t, portRedirect, false),
			},
			success: true,
		},
		{
			name: "L4 Single destination port with redirect and exclusion",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1}),
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionRedirect(t, portRedirect, false),
			},
			success: true,
		},
		{
			name: "L4 list of destination ports with redirects",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1, port2, port3}),
					},
				},
				Action: setActionRedirect(t, portRedirect, false),
			},
			success: true,
		},
		{
			name: "L4 list of destination ports with verdicts",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1, port2}),
					},
				},
				Action: setActionVerdict(t, unix.NFT_RETURN),
			},
			success: true,
		},
		{
			name: "L4 list of destination ports with redirects with exclude",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1, port2}),
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionRedirect(t, portRedirect, false),
			},
			success: true,
		},
		{
			name: "L4 list of destination ports with verdicts with exclude",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						List: nftableslib.SetPortList([]int{port1, port2}),
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionVerdict(t, unix.NFT_RETURN),
			},
			success: true,
		},
		{
			name: "L4 Range of destination ports with redirects",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						Range: nftableslib.SetPortRange([2]int{port1, port2}),
					},
				},
				Action: setActionRedirect(t, portRedirect, false),
			},
			success: true,
		},
		{
			name: "L4 Range of destination ports with verdicts",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						Range: nftableslib.SetPortRange([2]int{port1, port2}),
					},
				},
				Action: setActionVerdict(t, unix.NFT_RETURN),
			},
			success: true,
		},
		{
			name: "L4 range of destination ports with redirects with exclude",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						Range: nftableslib.SetPortRange([2]int{port1, port2}),
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionRedirect(t, portRedirect, false),
			},
			success: true,
		},
		{
			name: "L4 Range of destination ports with verdicts with exclude",
			rule: nftableslib.Rule{
				L4: &nftableslib.L4Rule{
					L4Proto: unix.IPPROTO_TCP,
					Dst: &nftableslib.Port{
						Range: nftableslib.SetPortRange([2]int{port1, port2}),
					},
					RelOp: nftableslib.NEQ,
				},
				Action: setActionVerdict(t, unix.NFT_RETURN),
			},
			success: true,
		},
	}
	m := InitMockConn()
	m.ti.Tables().Create("filter-v4", nftables.TableFamilyIPv4)
	tblV4, err := m.ti.Tables().Table("filter-v4", nftables.TableFamilyIPv4)
	if err != nil {
		t.Fatalf("failed to get chain interface for table filter-v4")
	}
	chainAttrs := nftableslib.ChainAttributes{
		Hook:     nftables.ChainHookInput,
		Type:     nftables.ChainTypeFilter,
		Priority: nftables.ChainPriorityFilter,
	}
	tblV4.Chains().Create("chain-1-v4", &chainAttrs)

	m.ti.Tables().Create("filter-v6", nftables.TableFamilyIPv6)
	tblV6, err := m.ti.Tables().Table("filter-v6", nftables.TableFamilyIPv6)
	if err != nil {
		t.Fatalf("failed to get chain interface for table filter-v6")
	}
	tblV6.Chains().Create("chain-1-v6", &chainAttrs)

	for _, tt := range ipv4Tests {
		ri, err := tblV4.Chains().Chain("chain-1-v4")
		if err != nil {
			t.Fatalf("failed to get rules interface for chain chain-1-v4")
		}
		_, err = ri.Rules().Create(&tt.rule)
		if err == nil && !tt.success {
			t.Errorf("Test: %s should fail but succeeded", tt.name)
		}
		if err != nil && tt.success {
			t.Errorf("Test: %s should succeed but fail with error: %v", tt.name, err)
		}
	}

	for _, tt := range ipv6Tests {
		ri, err := tblV6.Chains().Chain("chain-1-v6")
		if err != nil {
			t.Fatalf("failed to get rules interface for chain chain-1-v6")
		}
		_, err = ri.Rules().Create(&tt.rule)
		if err == nil && !tt.success {
			t.Errorf("Test: %s should fail but succeeded", tt.name)
		}
		if err != nil && tt.success {
			t.Errorf("Test: %s should succeed but fail with error: %v", tt.name, err)
		}
	}

	for _, tt := range l4PortTests {
		ri, err := tblV4.Chains().Chain("chain-1-v4")
		if err != nil {
			t.Fatalf("failed to get rules interface for chain chain-1-v4")
		}
		_, err = ri.Rules().Create(&tt.rule)
		if err == nil && !tt.success {
			t.Errorf("Test: %s should fail but succeeded", tt.name)
		}
		if err != nil && tt.success {
			t.Errorf("Test: %s should succeed but fail with error: %v", tt.name, err)
		}
	}

	for _, tt := range v2ipv4tests {
		ri, err := tblV4.Chains().Chain("chain-1-v4")
		if err != nil {
			t.Fatalf("failed to get rules interface for chain chain-1-v4")
		}
		_, err = ri.Rules().Create(&tt.rule)
		if err == nil && !tt.success {
			t.Errorf("Test: %s should fail but succeeded", tt.name)
		}
		if err != nil && tt.success {
			t.Errorf("Test: %s should succeed but fail with error: %v", tt.name, err)
		}
	}
	for _, tt := range v2ipv6tests {
		ri, err := tblV6.Chains().Chain("chain-1-v6")
		if err != nil {
			t.Fatalf("failed to get rules interface for chain chain-1-v6")
		}
		_, err = ri.Rules().Create(&tt.rule)
		if err == nil && !tt.success {
			t.Errorf("Test: %s should fail but succeeded", tt.name)
		}
		if err != nil && tt.success {
			t.Errorf("Test: %s should succeed but fail with error: %v", tt.name, err)
		}
	}

	if err := m.Flush(); err != nil {
		t.Errorf("Failed Flushing Tables with error: %v", err)
	}

	nft, _ := m.ti.Tables().Dump()

	t.Logf("Resulting tables: %s", string(nft))

}
