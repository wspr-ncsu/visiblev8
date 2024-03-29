package core

// utilities for loading/querying IDL interface/member data from a JSON store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

	"golang.org/x/exp/slices"
)

var GLOBAL_JS_OBJECTS = []string{
	// NON-STANDARD PROPERTIES
	`Console`,
	`Save`,
	`hasOwnProperty`,
	`setImmediate`,
	`setTimeout`,
	`clearImmediate`,
	// STANDARD PROPERTIES
	`globalThis`,
	`Infinity`,
	`NaN`,
	`undefined`,
	`eval`,
	`isFinite`,
	`isNaN`,
	`parseFloat`,
	`parseInt`,
	`decodeURI`,
	`decodeURIComponent`,
	`encodeURI`,
	`encodeURIComponent`,
	`escape`,
	`unescape`,
	`Object`,
	`Function`,
	`Boolean`,
	`Symbol`,
	`Error`,
	`AggregateError`,
	`EvalError`,
	`RangeError`,
	`ReferenceError`,
	`SyntaxError`,
	`TypeError`,
	`URIError`,
	`InternalError`,
	`Number`,
	`BigInt`,
	`Math`,
	`Date`,
	`Array`,
	`Int8Array`,
	`Uint8Array`,
	`Uint8ClampedArray`,
	`Int16Array`,
	`Uint16Array`,
	`Int32Array`,
	`Uint32Array`,
	`BigInt64Array`,
	`BigUint64Array`,
	`Float32Array`,
	`Float64Array`,
	`String`,
	`RegExp`,
	`Map`,
	`Set`,
	`WeakMap`,
	`WeakSet`,
	`ArrayBuffer`,
	`SharedArrayBuffer`,
	`DataView`,
	`Atomics`,
	`JSON`,
	`Iterator`,
	`AsyncIterator`,
	`Promise`,
	`GeneratorFunction`,
	`AsyncGeneratorFunction`,
	`Generator`,
	`AsyncGenerator`,
	`AsyncFunction`,
	`WeakRef`,
	`FinalizationRegistry`,
	`Intl`,
	`Reflect`,
	`Proxy`,
}

// IDLInterface is a JSON-unmarshalling structure for reading records from idldata.json
type IDLInterface struct {
	ParentName string   `json:"parent"`
	AliasFor   string   `json:"aliasFor"`
	Aliases    []string `json:"aliases"`
	Members    []string `json:"members"`
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
	sort.Strings(iface.Properties) // The arrays need to be sorted for sort.SearchStrings' index to make sense
	sort.Strings(iface.Methods)
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

func (tree IDLTree) IsAPIInIDLFile(op byte, class, member string) bool {
	if op == 'n' {
		_, ok := tree[member]
		if !ok {
			return false
		} else {
			return true
		}
	} else if op == 'c' || op == 'g' {
		// Probably a constructor initialization, being miscategorized as a call/get
		_, ok := tree[member]
		if ok {
			return true
		}

		idx := slices.IndexFunc(GLOBAL_JS_OBJECTS, func(elem string) bool { return elem == member })
		if idx != -1 {
			return true
		}
	}

	iface, ok := tree[class]
	if !ok {
		return false
	}

	memberSlot := slices.IndexFunc(iface.Members, func(elem string) bool { return elem == member })
	if memberSlot != -1 {
		return true
	}

	methodSlot := slices.IndexFunc(iface.Methods, func(elem string) bool { return elem == member })
	if methodSlot != -1 {
		return true
	}

	propertySlot := slices.IndexFunc(iface.Methods, func(elem string) bool { return elem == member })

	if propertySlot != -1 {
		return true
	}

	if tree.IsAPIInIDLFile(op, iface.ParentName, member) {
		return true
	}

	for _, alias := range iface.Aliases {
		if tree.IsAPIInIDLFile(op, alias, member) {
			return true
		}
	}

	return false
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
