# This works for versions post this commit https://source.chromium.org/chromium/chromium/src/+/c2339832313843e1b199d7d347be96f205dd39c4
# Run with python3 builder/resources/build/visiblev8_idl_generator.py --chrome-root /path/to/chromium/src

import sys
import os
import argparse

parser = argparse.ArgumentParser(description='Generate a list of IDL interfaces and their members.')
parser.add_argument('--chrome-root', required=True, help='Path to the root of the Chromium source tree.')
args = parser.parse_args()

sys.path.append(os.path.join(args.chrome_root,'third_party/blink/renderer/bindings/scripts'))

import web_idl
import json

web_idl_database_path = os.path.join( args.chrome_root, 'out/Release/gen/third_party/blink/renderer/bindings/web_idl_database.pickle' )
web_idl_database = web_idl.Database.read_from_file(web_idl_database_path)

idl_data = {}

# This is the list of interfaces that we want to dump.
# The list is taken from the list of interfaces that are exposed to JavaScript.
for interface in web_idl_database.interfaces:
    attributes = []
    for attr in interface.attributes:
        attributes.append(attr.identifier)
    operations = []
    for method in interface.operations:
        operations.append(method.identifier)
    aliases = []
    for alias in interface.legacy_window_aliases:
        aliases.append(alias.identifier)
    idl_data[interface.identifier] = {
        'members': attributes + operations,
        'properties': attributes,
        'methods': operations,
        'parent': interface.inherited.identifier if interface.inherited else None,
        'aliases': aliases
    }

print(json.dumps(idl_data, indent=2, sort_keys=True))
