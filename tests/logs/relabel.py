#!/usr/bin/env python3
from __future__ import print_function

"""VisibleV8 log relabeler: eliminates non-deterministic elements from vv8 logs for diffing.

E.g., transforms the log records

~0xdeadbeefbaadf00d
$42:function() { eval("while(true) {}"); }
!42
$43:while(true) {}
!43

into

~isolate0
$script0:function() { eval("while(true) {}"); }
!script0
$script1:while(true) {}
!script1
"""
import sys

import vlp


def relabel_volatile_log_fields(stream):
    """Yields log records with Isolate pointers and script IDs deterministically relabeled

    The first Isolate pointer encountered is relabeled "isolate0", the second "isolate1", etc.
    The first script ID encountered is relabeled "id0", "id1", etc.

    This allows deterministic semantic comparison of logs generated at different times by different builds of VisibleV8 (assuming identical source programs).
    """
    isolate_map = {}
    sid_map = {}
    obj_map = {}

    def relabel_isolate(isolate):
        try:
            return isolate_map[isolate]
        except KeyError:
            label = "isolate{0}".format(len(isolate_map))
            isolate_map[isolate] = label
            return label

    def relabel_sid(sid):
        try:
            return sid_map[sid]
        except KeyError:
            label = "script{0}".format(len(sid_map))
            sid_map[sid] = label
            return label

    def relabel_obj(oid):
        try:
            return obj_map[oid]
        except KeyError:
            label = "obj{0}".format(len(obj_map))
            obj_map[oid] = label
            return label

    def relabel_obj_field(field):
        if "," in field:
            obj_id = field[1 : len(field) - 1].split(",")[0]
            recv = field[1 : len(field) - 1].split(",")[1:]
            return "{{{0},{1}}}".format(relabel_obj(obj_id), ''.join(recv))
        return field

    for line in stream:
        line = line.strip()

        # Parse/relabel only "special" lines
        if line and line[0] in "~$!cgs":
            fields = vlp.parse_raw_fields(line[1:])

            if line[0] == "~":
                # Isolate pointers are only in ~ records
                fields[0] = relabel_isolate(fields[0])
            elif line[0] in "$!":
                # At least one SID...
                fields[0] = relabel_sid(fields[0])

                # Maybe two for '$' records...
                if (line[0] == "$") and (fields[1].isdigit()):
                    fields[1] = relabel_sid(fields[1])
            else:
                # Either a call/get/set operation
                # Do we have object ID tagging?
                for i in range(0,len(fields)):
                    fields[i] = relabel_obj_field(fields[i])

            # Repack and emit the relabeled line
            yield line[0] + vlp.pack_raw_fields(fields)
        else:
            # Emit normal records verbatim
            yield line


def test_relabel_volatile_log_fields():
    input_lines = """\
~0xbaadf00d
@?
$42:"foo.js":function() { eval("while(true) {}"); }
!42
$43:42:while(true) {}
!43""".split(
        "\n"
    )

    output_lines = """\
~isolate0
@?
$script0:"foo.js":function() { eval("while(true) {}"); }
!script0
$script1:script0:while(true) {}
!script1""".split(
        "\n"
    )

    stream = relabel_volatile_log_fields(line + "\n" for line in input_lines)
    for expected, actual in zip(stream, output_lines):
        assert expected == actual


def main(argv):
    for line in relabel_volatile_log_fields(sys.stdin):
        print(line)


if __name__ == "__main__":
    main(sys.argv)
