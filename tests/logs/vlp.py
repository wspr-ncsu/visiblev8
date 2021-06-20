#!/usr/bin/env python3
from __future__ import print_function

"""libVLP: VisibleV8 Log Parser Library
"""
import os


# Field delimiter handling (bi-directional)
###########################################


def parse_raw_fields(line):
    """Returns a list with one string per ':'-delimited field in a VisV8 log record.

    Properly handles backslash escapes of ':' delimiters, '\\' characters,
    and Unicode escapes...

    TOO SLOW FOR POST-PRODUCTION--useful only for testing/etc.
    """
    COPY, ESC, HEX, UNI = range(4)
    all_fields = []
    field = []
    scratch = ""
    state = COPY
    for c in line:
        if state == COPY:
            if c == "\\":
                state = ESC
            elif c == ":":
                all_fields.append("".join(field))
                field = []
            else:
                field.append(c)
        elif state == ESC:
            if c == "x":
                state = HEX
            elif c == "u":
                state = UNI
            else:
                field.append(c)
                state = COPY
        elif state == HEX:
            scratch += c
            if len(scratch) == 2:
                field.append(chr(int(scratch, 16)))
                scratch = ""
                state = COPY
        elif state == UNI:
            scratch += c
            if len(scratch) == 4:
                field.append(chr(int(scratch, 16)))
                scratch = ""
                state = COPY

    if field:
        all_fields.append("".join(field))

    return all_fields


def test_parse_raw_fields():
    """Unit tests for field parsing function."""
    tests = [
        ("no colons at all", ["no colons at all"]),
        ("no slashes : but colons", ["no slashes ", " but colons"]),
        (r"just \\ slashes\\\\ ", [r"just \ slashes\\ "]),
        ("escaped\:colons\:galore", ["escaped:colons:galore"]),
        (
            r'cprint:#U:"this is a string with \:colons\: and \\backslashes\\ in it"',
            [
                "cprint",
                "#U",
                r'"this is a string with :colons: and \backslashes\ in it"',
            ],
        ),
        (
            r'cprint:#U:"and this one is a \\\:doozy\\\:!!!"',
            ["cprint", "#U", r'"and this one is a \:doozy\:!!!"'],
        ),
        (
            r"this field ends with \\:and this one doesn't",
            ["this field ends with \\", "and this one doesn't"],
        ),
        (
            r"this field has an embedded newline here \x0a and here \x0a",
            ["this field has an embedded newline here \n and here \n"],
        ),
    ]

    for inp, out in tests:
        assert parse_raw_fields(inp) == out


def pack_raw_fields(fields):
    """Pack the strings in the list <fields> into a \\-escaped, :-delimited log record."""

    def xlat(c):
        if c in ":\\":
            return "\\" + c
        elif c < " ":
            return "\\x{0:02x}".format(ord(c))
        elif c > "~":
            return "\\u{0:04x}".format(ord(c))
        else:
            return c

    return ":".join("".join(xlat(c) for c in f) for f in fields)
