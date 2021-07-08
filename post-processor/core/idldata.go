package core

// utilities for loading/querying IDL interface/member data from a JSON store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
)

// IDLInterface is a JSON-unmarshalling structure for reading records from idldata.json
type IDLInterface struct {
	ParentName string `json:"parent"`
	AliasFor   string `json:"aliasFor"`
	//Members    []string `json:"members"`
	Methods    []string `json:"methods"`
	Properties []string `json:"properties"`
}

// IDLTree is a convenience type alias for a map-of-string-to-IDLInterface
type IDLTree map[string]*IDLInterface

// IDLInfo is information about a feature name obtained from an IDL database
type IDLInfo struct {
	BaseInterface string // The base-interface name for this feature
	MemberRole    rune   // The member role (p = property, m = method, a = alias)
}

// LookupInfo finds the base-interface and member-role information (if any) for a class/member name pair
func (tree IDLTree) LookupInfo(class, member string) (IDLInfo, error) {
	info := IDLInfo{BaseInterface: class, MemberRole: '?'}
	iface, ok := tree[info.BaseInterface]
	if !ok {
		return info, fmt.Errorf("no such interface name '%s'", info.BaseInterface)
	}
	for iface != nil {
		if iface.AliasFor != "" {
			info.BaseInterface = iface.AliasFor
			iface, ok = tree[info.BaseInterface]
			if !ok {
				return info, fmt.Errorf("no such interface name '%s'", info.BaseInterface)
			}
		} else if slot := sort.SearchStrings(iface.Properties, member); (slot < len(iface.Properties)) && iface.Properties[slot] == member {
			info.MemberRole = 'p'
			return info, nil
		} else if slot := sort.SearchStrings(iface.Methods, member); (slot < len(iface.Methods)) && iface.Methods[slot] == member {
			info.MemberRole = 'm'
			return info, nil
		} else if iface.ParentName != "" {
			info.BaseInterface = iface.ParentName
			iface, ok = tree[info.BaseInterface]
			if !ok {
				return info, fmt.Errorf("no such interface name '%s'", info.BaseInterface)
			}
		} else if member == "" {
			// rare case (so we don't worry about the time wasted searching above); object constructor role
			info.MemberRole = 'c'
			return info, nil
		} else {
			return info, fmt.Errorf("interface '%s' has no such member name '%s'", info.BaseInterface, member)
		}
	}
	return info, fmt.Errorf("UNPOSSIBLE")
}

// NormalizeMember attempts to look up the base class/interface defining "member" and replace "class" with that name (for backwards compatability, basically)
func (tree IDLTree) NormalizeMember(class, member string) (string, error) {
	info, err := tree.LookupInfo(class, member)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", info.BaseInterface, member), nil
}

// LoadIDLData loads a idldata.json-like file into an IDLTree
func LoadIDLData(path string) (IDLTree, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tree IDLTree
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&tree)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// LoadDefaultIDLData is a sanity-retainer, so that all aggregators use the same ENV variable
func LoadDefaultIDLData() (IDLTree, error) {
	return LoadIDLData(GetEnvDefault("IDLDATA_FILE", "idldata.json"))
}

// IDLTreeTest is a horribly broken test since I lost wat.json...
func IDLTreeTest() {
	tree, err := LoadIDLData("wat.json")
	if err != nil {
		log.Fatal(err)
	}

	name, err := tree.NormalizeMember("Window", "barf")
	if err != nil {
		log.Print(err)
	} else {
		log.Printf("Normalized Window.barf -> %s", name)
	}
	name, err = tree.NormalizeMember("HTMLFormElement", "parentNode")
	if err != nil {
		log.Print(err)
	} else {
		log.Printf("Normalized HTMLFormElement.parentNode -> %s", name)
	}
	name, err = tree.NormalizeMember("Window", "addEventListener")
	if err != nil {
		log.Print(err)
	} else {
		log.Printf("Normalized Window.addEventListener -> %s", name)
	}
}
