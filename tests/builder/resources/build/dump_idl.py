#!/usr/bin/python
'''Extract essential interface information out of the Blink/WebKit IDL files (using their tooling).
'''
import fnmatch
import json
import os
import sys


VARIANT_MAP = {
    'chrome64': {
        'WebKit': "WebKit",
        'Source': "Source",
    },
    'chrome68': {
        'WebKit': "blink",
        'Source': "renderer",
    }
}

VARIANT = None


def chrome68_idl_implementations(idl_data):
    for imp in idl_data.implements:
        yield (imp.left_interface, imp.right_interface)


def chrome75_idl_implementations(idl_data):
    for imp in idl_data.includes:
        yield (imp.interface, imp.mixin)


def idl_implementations(idl_data):
    if hasattr(idl_data, 'includes'):
        return chrome75_idl_implementations(idl_data)
    elif hasattr(idl_data, 'implements'):
        return chrome68_idl_implementations(idl_data)
    else:
        raise Error("cannot determine IDL interface implementations from %r" % idl_data)


def collect_idl_files(webkit_root):
    from idl_reader import IdlReader
    rdr = IdlReader()

    idl_map = {}
    imp_map = {}
    alias_map = {}
    fcount = 0
    for part in ["core", "modules"]:
        search_root = os.path.join(webkit_root, VARIANT['Source'], part)
        for node, dirs, files in os.walk(search_root):
            for idl in fnmatch.filter(files, "*.idl"):
                idl_path = os.path.join(node, idl)
                if '/testing/' in idl_path:
                    print >>sys.stderr, "Ignoring 'testing' IDL file '%s'" % idl_path
                    continue
                idl_data = rdr.read_idl_file(idl_path)
                fcount += 1
                for imp_left, imp_right in idl_implementations(idl_data):
                    try:
                        imp_map[imp_left].add(imp_right)
                    except KeyError:
                        imp_map[imp_left] = set([imp_right])
                for iface, idef in idl_data.interfaces.items():
                    if 'NamedConstructor' in idef.extended_attributes:
                        alias_map[idef.extended_attributes['NamedConstructor']] = iface
                    try:
                        idl_list = idl_map[iface]
                        if idef.is_partial:
                            idl_list.append((idl_path, idef))
                        else:
                            idl_list.insert(0, (idl_path, idef))
                    except KeyError:
                        idl_map[iface] = [(idl_path, idef)]

    print >>sys.stderr, "Processed %d IDL files" % fcount
    return idl_map, imp_map, alias_map


def dump_interfaces(idl_map, imp_map, alias_map):
    true_imap = {}
    
    # Merge pieces of interfaces split across IDL files implicitly
    for iname, ideflist in idl_map.items():
        _, idef = ideflist[0]
        for _, p in ideflist[1:]:
            idef.merge(p)
        true_imap[iname] = idef

    # Handle explicit interface implementation
    for iname, idef in true_imap.items():
        for impi in imp_map.get(iname, []):
            idef.merge(true_imap[impi])

    # Dump
    tree = {}
    for iname, idef in true_imap.items():
	# This was our original bug
        #if 'NoInterfaceObject' in idef.extended_attributes:
        #    continue
    	# And this turned out to be a bad idea, too
        #if len(idef.attributes + idef.operations) == 0:
        #    continue

        tree[iname] = {
            'parent': idef.parent,
            'members': list(sorted({x.name for x in idef.attributes + idef.operations if x.name})),
            'properties': list(sorted({a.name for a in idef.attributes if a.name})),
            'methods': list(sorted({op.name for op in idef.operations if op.name})),
        }
        #print json.dumps(['iface', idef.name, idef.parent])

        #attrs = [(a.name, a) for a in idef.attributes]
        #for aname, attr in sorted(attrs):
            #print "%s.%s -> %s" % (iname, aname, attr.idl_type.name)

        #ops = [(o.name, o) for o in idef.operations]
        #for oname, op in sorted(ops):
        #    arg_types = ', '.join('%s %s' % (a.idl_type.name, a.name)
        #                            for a in op.arguments)
        #    print "%s.%s(%s) -> %s" % (iname, oname or '[]',
        #                                arg_types, op.idl_type.name)

    for named_ctor, iface in alias_map.items():
        tree[named_ctor] = {
            'aliasFor': iface,
        }

    json.dump(tree, sys.stdout)


def setup_path(chrome_root):
    global VARIANT

    if not os.path.isdir(chrome_root):
        print >>sys.stderr, "error: %s is not a directory" % (chrome_root, )
        sys.exit(1)
    
    for vname, variant in VARIANT_MAP.items():
        webkit_root = os.path.join(chrome_root, "third_party", variant['WebKit'])
        if not os.path.isdir(webkit_root):
            continue
        script_dir = os.path.join(webkit_root, variant['Source'], "bindings", "scripts")
        if os.path.isfile(os.path.join(script_dir, "idl_reader.py")):
            sys.path.append(os.path.join(script_dir)) 
            VARIANT = variant
            print >>sys.stderr, "found chrome source tree variant '%s'" % vname
            break

    if not VARIANT:
        print >>sys.stderr, "error: couldn't find necessary components under '%s'; are you sure it's chromium?" % chrome_root
        sys.exit(1)
    
    return webkit_root


def main(argv):
    if len(argv) < 2:
        print >>sys.stderr, "usage: %s CHROME_SOURCE_ROOT" % (argv[0],)
        sys.exit(1)

    chrome_root = argv[1]
    webkit_root = setup_path(chrome_root)

    idl_map, imp_map, alias_map = collect_idl_files(webkit_root)
    #print >> sys.stderr, '\n'.join(sorted(idl_map.keys()))
    dump_interfaces(idl_map, imp_map, alias_map)


if __name__ == "__main__":
    main(sys.argv)

