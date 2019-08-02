package nftableslib

import (
	"fmt"
	"math/rand"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func createL3(l3proto nftables.TableFamily, rule *Rule) ([]expr.Any, []*nfSet, error) {
	re := []expr.Any{}
	e := []expr.Any{}
	sets := make([]*nfSet, 0)
	var set []*nfSet
	var err error

	// Processing non-nil keys defined in L3 portion of a rule
	if rule.L3.Version != nil {
		if e, _, err = processVersion(*rule.L3.Version, rule.Exclude); err != nil {
			return nil, nil, err
		}
		re = append(re, e...)
	}

	if rule.L3.Protocol != nil {
		if e, _, err = processProtocol(l3proto, *rule.L3.Protocol, rule.Exclude); err != nil {
			return nil, nil, err
		}
		re = append(re, e...)
	}

	if rule.L3.Src != nil {
		if e, set, err = processIPAddr(l3proto, rule.L3.Src, true, rule.Exclude); err != nil {
			return nil, nil, err
		}
		if set != nil {
			sets = append(sets, set...)
		}
		re = append(re, e...)
	}

	if rule.L3.Dst != nil {
		if e, set, err = processIPAddr(l3proto, rule.L3.Dst, false, rule.Exclude); err != nil {
			return nil, nil, err
		}
		if set != nil {
			sets = append(sets, set...)
		}
		re = append(re, e...)
	}
	return re, sets, nil
}

func processAddrList(l3proto nftables.TableFamily, offset uint32, list []*IPAddr,
	excl bool) ([]expr.Any, *nfSet, error) {

	nfset := &nfSet{}
	set := &nftables.Set{
		Anonymous: false,
		Constant:  true,
		Name:      getSetName(),
		ID:        uint32(rand.Intn(0xffff)),
	}
	se := make([]nftables.SetElement, len(list))
	if l3proto == nftables.TableFamilyIPv4 {
		for i := 0; i < len(list); i++ {
			se[i].Key = list[i].IP.To4()
		}
	}
	if l3proto == nftables.TableFamilyIPv6 {
		for i := 0; i < len(list); i++ {
			se[i].Key = list[i].IP.To16()
		}
	}
	if len(se) == 0 {
		return nil, nil, fmt.Errorf("unknown nftables.TableFamily %#02x", l3proto)
	}
	nfset.set = set
	nfset.elements = se
	re, err := getExprForListIPV2(l3proto, set, offset, excl)
	if err != nil {
		return nil, nil, err
	}

	return re, nfset, nil
}

func processAddrRange(l3proto nftables.TableFamily, offset uint32, rng [2]*IPAddr, excl bool) ([]expr.Any, *nfSet, error) {
	re, err := getExprForRangeIP(l3proto, offset, rng, excl)
	if err != nil {
		return nil, nil, err
	}

	return re, nil, nil
}

func processVersion(version byte, excl bool) ([]expr.Any, *nfSet, error) {
	re, err := getExprForIPVersion(version, excl)
	if err != nil {
		return nil, nil, err
	}

	return re, nil, nil
}

func processProtocol(l3proto nftables.TableFamily, proto uint32, excl bool) ([]expr.Any, *nfSet, error) {
	re, err := getExprForProtocol(l3proto, proto, excl)
	if err != nil {
		return nil, nil, err
	}

	return re, nil, nil
}

func processIPAddr(l3proto nftables.TableFamily, addrs *IPAddrSpec, src bool, exclude bool) ([]expr.Any, []*nfSet, error) {
	var addrOffset uint32
	var keyType nftables.SetDatatype
	var set *nfSet
	var err error
	sets := make([]*nfSet, 0)
	e := []expr.Any{}
	re := []expr.Any{}
	switch l3proto {
	case nftables.TableFamilyIPv4:
		if src {
			addrOffset = 12
		} else {
			addrOffset = 16
		}
		keyType = nftables.TypeIPAddr
	case nftables.TableFamilyIPv6:
		if src {
			addrOffset = 8
		} else {
			addrOffset = 24
		}
		keyType = nftables.TypeIP6Addr
	}
	// If list is not nil processing elements
	if addrs.List != nil {
		if e, set, err = processAddrList(l3proto, addrOffset, addrs.List, exclude); err != nil {
			return nil, nil, err
		}
		if set != nil {
			set.set.KeyType = keyType
			sets = append(sets, set)
		}
		re = append(re, e...)
	}
	// if both elements of the range are specified, processing elements
	if addrs.Range[0] != nil && addrs.Range[1] != nil {
		if e, set, err = processAddrRange(l3proto, addrOffset, addrs.Range, exclude); err != nil {
			return nil, nil, err
		}
		if set != nil {
			set.set.KeyType = keyType
			sets = append(sets, set)
		}
		re = append(re, e...)
	}

	return re, sets, nil
}
