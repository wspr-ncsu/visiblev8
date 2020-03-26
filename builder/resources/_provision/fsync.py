#!/usr/bin/env python3
"""fsync.py: force gclient sync on release commits by force-fetching missing commits within submodules

Workaround to the sad fact that the stock depot_tools commands don't facilitate checkout out and building
release branches/tags of Chromium.
"""
import os
import subprocess
import sys
import re


GCLIENT = "gclient"
GIT = "git"


MISSING_OBJECT = re.compile(r"Error: Command 'git checkout --quiet (?P<hash>[0-9a-f]{40})' returned non-zero exit status \d+ in (?P<path>.*)\
fatal: reference is not a tree: \1")

UNSTAGED_CHANGES = re.compile(r"\s+(?P<path>\S+) \(ERROR\).*\1 at [0-9a-f]{40}.*You have unstaged changes.", re.DOTALL)


def reset_entire_tree(root_path):
    for node, dirs, files in os.walk(root_path):
        if ".git" in dirs:
            reset_dirty_subtree(node)
            dirs.remove('.git')


def reset_dirty_subtree(path):
    print("{0}: performing git reset --hard".format(path))
    old = os.getcwd()
    os.chdir(path)
    try:
        subprocess.check_call([GIT, "reset", "--hard", "--quiet"])
    finally:
        os.chdir(old)


def try_sync():
    try:
        subprocess.check_output([GCLIENT, "sync",
            #"--disable-syntax-validation",
            "--no-history",
            "--with_branch_heads",
            "--with_tags",
            "--reset",
            "--shallow"], stderr=subprocess.STDOUT)
        return True, None, None
    except subprocess.CalledProcessError as err:
        output = err.output.decode('utf-8')

    m = MISSING_OBJECT.search(output)
    if m:
        return False, fetch_missing_ref, [m.group("path"), m.group("hash")]
    else:
        raise Exception(output)


def fetch_missing_ref(path, ref):
    print("{0}: fetching {1}".format(path, ref))
    old = os.getcwd()
    os.chdir(path)
    try:
        subprocess.check_call([GIT, "fetch", "--quiet", "origin", ref])
    finally:
        os.chdir(old)


def main(argv):
    #reset_entire_tree('.')
    ok, fixer, args = try_sync()
    while not ok:
        fixer(*args)
        ok, fixer, args = try_sync()


if __name__ == "__main__":
    main(sys.argv)

