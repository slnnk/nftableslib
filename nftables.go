package nftableslib

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/google/nftables"
	"golang.org/x/sys/unix"
)

// TablesInterface defines a top level interface
type TablesInterface interface {
	Tables() TableFuncs
}

// TableFuncs defines second level interface operating with nf tables
type TableFuncs interface {
	Table(name string, familyType nftables.TableFamily) (ChainsInterface, error)
	TableChains(name string, familyType nftables.TableFamily) (ChainsInterface, error)
	TableSets(name string, familyType nftables.TableFamily) (SetsInterface, error)
	Create(name string, familyType nftables.TableFamily) error
	Delete(name string, familyType nftables.TableFamily) error
	CreateImm(name string, familyType nftables.TableFamily) error
	DeleteImm(name string, familyType nftables.TableFamily) error
	Exist(name string, familyType nftables.TableFamily) bool
	Get(familyType nftables.TableFamily) ([]string, error)
	Sync(familyType nftables.TableFamily) error
	Dump() ([]byte, error)
}

type nfTables struct {
	conn NetNS
	sync.Mutex
	// Two dimensional map, 1st key is table family, 2nd key is table name
	tables map[nftables.TableFamily]map[string]*nfTable
}

// nfTable defines a single type/name nf table with its linked chains
type nfTable struct {
	table *nftables.Table
	ChainsInterface
	SetsInterface
}

// Tables returns methods available for managing nf tables
func (nft *nfTables) Tables() TableFuncs {
	return nft
}

// Table returns Chains Interface for a specific table
func (nft *nfTables) Table(name string, familyType nftables.TableFamily) (ChainsInterface, error) {
	nft.Lock()
	defer nft.Unlock()
	// Check if nf table with the same family type and name  already exists
	if t, ok := nft.tables[familyType][name]; ok {
		return t.ChainsInterface, nil

	}

	return nil, fmt.Errorf("table %s of type %v does not exist", name, familyType)
}

// TableChains returns Chains Interface for a specific table
func (nft *nfTables) TableChains(name string, familyType nftables.TableFamily) (ChainsInterface, error) {
	nft.Lock()
	defer nft.Unlock()
	// Check if nf table with the same family type and name  already exists
	if t, ok := nft.tables[familyType][name]; ok {
		return t.ChainsInterface, nil

	}

	return nil, fmt.Errorf("table %s of type %v does not exist", name, familyType)
}

// TableChains returns Chains Interface for a specific table
func (nft *nfTables) TableSets(name string, familyType nftables.TableFamily) (SetsInterface, error) {
	nft.Lock()
	defer nft.Unlock()
	// Check if nf table with the same family type and name  already exists
	if t, ok := nft.tables[familyType][name]; ok {
		return t.SetsInterface, nil

	}

	return nil, fmt.Errorf("table %s of type %v does not exist", name, familyType)
}

// Create appends a table into NF tables list
func (nft *nfTables) Create(name string, familyType nftables.TableFamily) error {
	nft.Lock()
	defer nft.Unlock()
	nft.conn.AddTable(nft.create(name, familyType).table)

	return nil
}

func (nft *nfTables) create(name string, familyType nftables.TableFamily) *nfTable {
	// Check if tableFamily already allocated
	if _, ok := nft.tables[familyType]; ok {
		// Check if table  already exists
		if _, ok := nft.tables[familyType][name]; ok {
			// Check if table has ChainsInterface and SetsInterface instantiated
			if nft.tables[familyType][name].ChainsInterface != nil && nft.tables[familyType][name].SetsInterface != nil {
				// Table already exists with proper interfaces, no need to do anything
				return nft.tables[familyType][name]
			}
		}
	} else {
		// First table for familyType, allocating memory
		nft.tables[familyType] = make(map[string]*nfTable)
	}

	t := &nftables.Table{
		Family: familyType,
		Name:   name,
	}
	nft.tables[familyType][name] = &nfTable{
		table:           t,
		ChainsInterface: newChains(nft.conn, t),
		SetsInterface:   newSets(nft.conn, t),
	}

	return nft.tables[familyType][name]
}

// Create appends a table into NF tables list and request to program it immediately
func (nft *nfTables) CreateImm(name string, familyType nftables.TableFamily) error {
	nft.Lock()
	defer nft.Unlock()
	nft.conn.AddTable(nft.create(name, familyType).table)
	err := nft.conn.Flush()
	// If the error indicates that the table already exists, then consider it as a non error
	if errors.Is(err, unix.EEXIST) {
		return nil
	}

	return err
}

// DeleteImm requests nftables module to remove a specified table from the kernel and from NF tables list
func (nft *nfTables) DeleteImm(name string, familyType nftables.TableFamily) error {
	if err := nft.Delete(name, familyType); err != nil {
		return err
	}

	return nft.conn.Flush()
}

// Delete removes a specified table from NF tables list
func (nft *nfTables) Delete(name string, familyType nftables.TableFamily) error {
	nft.Lock()
	defer nft.Unlock()
	// Check if nf table with the same family type and name  already exists
	if _, ok := nft.tables[familyType][name]; ok {
		// Removing old table, at this point, this table should be removed from the kernel as well.
		delete(nft.tables[familyType], name)
	}
	if nft.Tables().Exist(name, familyType) {
		nft.conn.DelTable(&nftables.Table{
			Name:   name,
			Family: familyType,
		})
	}
	// If no more tables exists under a specific family name, removing  family type.
	if len(nft.tables[familyType]) == 0 {
		delete(nft.tables, familyType)
	}

	return nil
}

// Exist checks is the table already defined
func (nft *nfTables) Exist(name string, familyType nftables.TableFamily) bool {
	// Check if Table exists in the store
	if _, ok := nft.tables[familyType][name]; ok {
		return true
	}
	// It is not in the store, let's double check if it exists on the host
	tables, err := nft.get(familyType)
	if err != nil {
		return false
	}
	for _, table := range tables {
		if table == name {
			return true
		}
	}

	return false
}

// Get returns all tables defined for a specific TableFamily
func (nft *nfTables) Get(familyType nftables.TableFamily) ([]string, error) {
	nft.Lock()
	defer nft.Unlock()

	return nft.get(familyType)
}

func (nft *nfTables) get(familyType nftables.TableFamily) ([]string, error) {
	nftables, err := nft.conn.ListTables()
	if err != nil {
		return nil, err
	}

	var tables []string
	for _, t := range nftables {
		if t.Family == familyType {
			tables = append(tables, t.Name)
		}
	}

	return tables, nil
}

// Sync synchronizes tables defined on the host with tables store, newly discovered
// tables will be added, stale will be removed fomr the store.
func (nft *nfTables) Sync(familyType nftables.TableFamily) error {
	nft.Lock()
	nftables, err := nft.conn.ListTables()
	if err != nil {
		return err
	}
	nft.Unlock()

	// Getting  list of tables defined on the host
	for _, t := range nftables {
		if t.Family == familyType {
			if _, ok := nft.tables[familyType][t.Name]; !ok {
				nt := nft.create(t.Name, t.Family)
				// Sync synchronizes all chains discovered in the table
				if err := nt.Chains().Sync(); err != nil {
					return err
				}
				// Sync synchronizes all sets discovered in the table
				if err := nt.Sets().Sync(); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Dump outputs json representation of all defined tables/chains/rules
func (nft *nfTables) Dump() ([]byte, error) {
	nft.Lock()
	defer nft.Unlock()
	var data []byte

	for _, f := range nft.tables {
		for _, t := range f {
			if b, err := json.Marshal(&t.table); err != nil {
				return nil, err
			} else {
				data = append(data, b...)
			}
			if b, err := t.Chains().Dump(); err != nil {
				return nil, err
			} else {
				data = append(data, b...)
			}
		}
	}

	return data, nil
}

func printTable(t *nftables.Table) []byte {
	return []byte(fmt.Sprintf("\nTable: %s Family: %+v Flags: %x Use: %x \n", t.Name, t.Family, t.Flags, t.Use))
}

// IsNFTablesOn detects whether nf_tables module is loaded or not, it return true is ListChains call succeeds,
// otherwise it return false.
func IsNFTablesOn() bool {
	conn := InitConn()
	if _, err := conn.ListChains(); err != nil {
		return false
	}
	return true
}
