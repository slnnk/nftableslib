package nftableslib

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
)

// SetAttributes  defines parameters of a nftables Set
type SetAttributes struct {
	Name       string
	Constant   bool
	IsMap      bool
	HasTimeout bool
	Timeout    time.Duration
	// Interval flag must be set only when the set elements are ranges, address ranges or port ranges
	Interval bool
	KeyType  nftables.SetDatatype
	DataType nftables.SetDatatype
}

// ElementValue defines key:value of the element of the type nftables.TypeIPAddr
// if IPAddrElement is element of a basic set, then only Addr will be specified,
// if it is element of a map then either Port or AddrIP and if it is element of a vmap, then
// Verdict.
type ElementValue struct {
	Addr   string
	Port   *uint16
	AddrIP *string
	Action *RuleAction
	// New members
	Integer     *uint32
	IPAddr      []byte
	EtherAddr   []byte
	InetProto   *byte
	InetService *uint16
	Mark        *uint32
}

// SetsInterface defines third level interface operating with nf maps
type SetsInterface interface {
	Sets() SetFuncs
}

// SetFuncs defines funcations to operate with nftables Sets
type SetFuncs interface {
	CreateSet(*SetAttributes, []nftables.SetElement) (*nftables.Set, error)
	DelSet(string) error
	GetSets() ([]*nftables.Set, error)
	GetSetByName(string) (*nftables.Set, error)
	GetSetElements(string) ([]nftables.SetElement, error)
	SetAddElements(string, []nftables.SetElement) error
	SetDelElements(string, []nftables.SetElement) error
	Sync() error
}

type nfSets struct {
	conn  NetNS
	table *nftables.Table
	sync.Mutex
	sets map[string]*nftables.Set
}

// Sets return a list of methods available for Sets operations
func (nfs *nfSets) Sets() SetFuncs {
	return nfs
}

func (nfs *nfSets) CreateSet(attrs *SetAttributes, elements []nftables.SetElement) (*nftables.Set, error) {
	var err error
	// TODO Add parameters validation
	se := []nftables.SetElement{}
	if attrs.Interval {
		if attrs.KeyType == nftables.TypeIPAddr || attrs.KeyType == nftables.TypeIP6Addr {
			if nfs.table.Family == nftables.TableFamilyIPv4 {
				se = append(se, nftables.SetElement{Key: net.ParseIP("0.0.0.0").To4(), IntervalEnd: true})
			} else {
				se = append(se, nftables.SetElement{Key: net.ParseIP("::").To16(), IntervalEnd: true})
			}
		}
	}
	s := &nftables.Set{
		Table:      nfs.table,
		ID:         uint32(rand.Intn(0xffff)),
		Name:       attrs.Name,
		Anonymous:  false,
		Constant:   attrs.Constant,
		Interval:   attrs.Interval,
		IsMap:      attrs.IsMap,
		HasTimeout: attrs.HasTimeout,
		KeyType:    attrs.KeyType,
		DataType:   attrs.DataType,
	}
	if attrs.HasTimeout && attrs.Timeout != 0 {
		// Netlink expects timeout in milliseconds
		s.Timeout = attrs.Timeout
	}
	// Adding elements to new Set if any provided
	se = append(se, elements...)
	if err = nfs.conn.AddSet(s, elements); err != nil {
		return nil, err
	}
	// Requesting Netfilter to programm it.
	if err := nfs.conn.Flush(); err != nil {
		return nil, err
	}
	nfs.Lock()
	defer nfs.Unlock()
	nfs.sets[attrs.Name] = s

	return s, nil
}

// Exist check if the set with name exists in the store and programmed on the host,
// if both checks succeed, true is returned, otherwise false is returned.
func (nfs *nfSets) Exist(name string) bool {
	nfs.Lock()
	_, ok := nfs.sets[name]
	nfs.Unlock()
	if !ok {
		return false
	}
	_, err := nfs.conn.GetSetByName(nfs.table, name)
	if err != nil {
		return false
	}

	return true
}

func (nfs *nfSets) GetSetByName(name string) (*nftables.Set, error) {
	nfs.Lock()
	_, ok := nfs.sets[name]
	nfs.Unlock()
	if !ok {
		return nil, fmt.Errorf("set %s is not found", name)
	}
	s, err := nfs.conn.GetSetByName(nfs.table, name)
	if err != nil {
		return nil, fmt.Errorf("set %s is not found", name)
	}

	return s, nil
}

func (nfs *nfSets) DelSet(name string) error {
	if nfs.Exist(name) {
		nfs.conn.DelSet(nfs.sets[name])
		if err := nfs.conn.Flush(); err != nil {
			return err
		}
		nfs.Lock()
		defer nfs.Unlock()
		delete(nfs.sets, name)
	}

	return nil
}

// GetSets returns a slice programmed on the host for a specific table.
func (nfs *nfSets) GetSets() ([]*nftables.Set, error) {
	return nfs.conn.GetSets(nfs.table)
}

func (nfs *nfSets) GetSetElements(name string) ([]nftables.SetElement, error) {
	if nfs.Exist(name) {
		return nfs.conn.GetSetElements(nfs.sets[name])
	}
	return nil, fmt.Errorf("set %s does not exist", name)
}

func (nfs *nfSets) SetAddElements(name string, elements []nftables.SetElement) error {
	if nfs.Exist(name) {
		if err := nfs.conn.SetAddElements(nfs.sets[name], elements); err != nil {
			return err
		}
		if err := nfs.conn.Flush(); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("set %s does not exist", name)
}

func (nfs *nfSets) SetDelElements(name string, elements []nftables.SetElement) error {
	if nfs.Exist(name) {
		set := nfs.sets[name]
		if err := nfs.conn.SetDeleteElements(set, elements); err != nil {
			return err
		}
		if err := nfs.conn.Flush(); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("set %s does not exist", name)
}

func (nfs *nfSets) Sync() error {
	sets, err := nfs.conn.GetSets(nfs.table)
	if err != nil {
		return err
	}
	for _, set := range sets {
		if _, ok := nfs.sets[set.Name]; !ok {
			nfs.Lock()
			nfs.sets[set.Name] = set
			nfs.Lock()
		}
	}

	return nil
}

func newSets(conn NetNS, t *nftables.Table) SetsInterface {
	return &nfSets{
		conn:  conn,
		table: t,
		sets:  make(map[string]*nftables.Set),
	}
}

// MakeElement creates a list of Elements for IPv4 or IPv6 address, slice of IPAddrElement
// carries IP address which will be used as a key in the element, and 3 possible values depending on the
// type of a set. Value could be IP address as a string, Port as uint16 and a nftables.Verdict
// For IPv4 addresses ipv4 bool should be set to true, otherwise IPv6 addresses are expected.
func MakeElement(input *ElementValue) ([]nftables.SetElement, error) {
	addr, err := NewIPAddr(input.Addr)
	if err != nil {
		return nil, err
	}

	// TODO Figure out if overlapping and possibility of collapsing needs to be checked.
	elements := buildElementRanges([]*IPAddr{addr})
	p := &elements[0]
	switch {
	case input.AddrIP != nil:
		valAddr, err := NewIPAddr(*input.AddrIP)
		if err != nil {
			return nil, err
		}
		// Checking that both key and value were of the same Family ether IPv4 or IPv6
		if addr.IsIPv6() {
			if !valAddr.IsIPv6() {
				return nil, fmt.Errorf("cannot mix ipv4 and ipv6 addresses in the same element")
			}
		}
		if !addr.IsIPv6() {
			if valAddr.IsIPv6() {
				return nil, fmt.Errorf("cannot mix ipv4 and ipv6 addresses in the same element")
			}
		}
		p.Val = valAddr.IP
	case input.Port != nil:
		p.Val = binaryutil.BigEndian.PutUint16(*input.Port)
	case input.Action != nil:
		p.VerdictData = input.Action.verdict
	}

	return elements, nil
}

// MakeConcatElement creates an element of a set/map as a concatination of standard SetDatatypes
// example: nftables.TypeIPAddr and nftables.TypeInetService
func MakeConcatElement(keys []nftables.SetDatatype,
	vals []ElementValue, ra *RuleAction) (*nftables.SetElement, error) {
	if ra == nil {
		return nil, fmt.Errorf("verdict cannot be nil")
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("number of keys cannot be 0")
	}
	if len(keys) != len(vals) {
		return nil, fmt.Errorf("number of vals does not match number of keys")
	}
	element := nftables.SetElement{}
	var key []byte
	var kl int
	for i := 0; i < len(keys); i++ {
		b, err := processElementValue(keys[i], vals[i])
		if err != nil {
			return nil, err
		}
		key = append(key, b...)
		kl += len(b)
	}

	// Make sure the slice is aligned to 4 bytes
	if kl%4 != 0 {
		kl += 4 - (kl % 4)
	}
	element.Key = make([]byte, kl)
	copy(element.Key, key)
	element.VerdictData = ra.verdict

	return &element, nil
}

func processElementValue(keyT nftables.SetDatatype, keyV ElementValue) ([]byte, error) {
	var b []byte
	switch keyT {
	case nftables.TypeInteger:
		if keyV.Integer == nil {
			return nil, fmt.Errorf("key value cannot be nil")
		}
		b = binaryutil.BigEndian.PutUint32(*keyV.Integer)
	case nftables.TypeMark:
		if keyV.Mark == nil {
			return nil, fmt.Errorf("key value cannot be nil")
		}
		b = binaryutil.BigEndian.PutUint32(*keyV.Mark)
	case nftables.TypeIPAddr:
		fallthrough
	case nftables.TypeIP6Addr:
		if keyV.IPAddr == nil {
			return nil, fmt.Errorf("key value cannot be nil")
		}
		b = make([]byte, len(keyV.IPAddr))
		copy(b, []byte(keyV.IPAddr))
	case nftables.TypeEtherAddr:
		if keyV.EtherAddr == nil {
			return nil, fmt.Errorf("key value cannot be nil")
		}
		b = make([]byte, len(keyV.EtherAddr))
		copy(b, []byte(keyV.EtherAddr))
	case nftables.TypeInetProto:
		if keyV.InetProto == nil {
			return nil, fmt.Errorf("key value cannot be nil")
		}
		b = []byte{*keyV.InetProto}
	case nftables.TypeInetService:
		if keyV.InetService == nil {
			return nil, fmt.Errorf("key value cannot be nil")
		}
		b = binaryutil.BigEndian.PutUint16(*keyV.InetService)
	default:
		return nil, fmt.Errorf("unsupported type of key element %d", keyT.GetNFTMagic())
	}
	// Alignment to 4 bytes
	l := len(b)
	if l%4 != 0 {
		l += 4 - (l % 4)
	}
	ba := make([]byte, l)
	copy(ba, b)

	return ba, nil
}

// GenSetKeyType generates a composite key type, combining all types
func GenSetKeyType(types ...nftables.SetDatatype) nftables.SetDatatype {
	newDatatype := nftables.SetDatatype{}
	switch len(types) {
	case 0:
		return nftables.TypeInvalid
	case 1:
		newDatatype.Name = types[0].Name
		newDatatype.SetNFTMagic(types[0].GetNFTMagic())
		if types[0].Bytes <= 4 {
			newDatatype.Bytes = 4
		} else {
			if types[0].Bytes%4 != 0 {
				newDatatype.Bytes = types[0].Bytes
				newDatatype.Bytes += 4 - (types[0].Bytes % 4)
			} else {
				newDatatype.Bytes = types[0].Bytes
			}
		}
		return newDatatype
	default:
		var c, b uint32
		name := "concat"
		for i := 0; i < len(types); i++ {
			name += "_" + types[i].Name
			c = c<<nftables.SetConcatTypeBits | types[i].GetNFTMagic()
			if types[i].Bytes <= 4 {
				b += 4
			} else {
				b += types[i].Bytes
				if types[i].Bytes%4 != 0 {
					b += 4 - (types[i].Bytes % 4)
				}
			}
			name += types[i].Name
			if i < len(types) {
				name += "_"
			}
		}
		newDatatype.Name = name
		newDatatype.Bytes = b
		newDatatype.SetNFTMagic(c)

		return newDatatype
	}
}
