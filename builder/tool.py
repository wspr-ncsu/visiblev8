#!/usr/bin/env python3
'''VisibleV8 Builder Tool

Actually a generic chromium-building tool that facilitates checking out 
specific [release] revisions of chromium, building selected targets in
a checked-out source tree, and installing artifacts from a build into
a fresh Docker container image.
'''
import argparse
import glob
import logging
import os
import re
import shutil
import subprocess
import sys
import tempfile


RESOURCE_DIR = os.path.join(os.path.dirname(__file__), "resources")

RE_COMMIT_HASHISH = re.compile(r'^[0-9a-fA-F]+$')

DOCKER_MAJOR_VERSION = 18
RE_DOCKER_MAJOR_VERSION = re.compile(r'^Docker version (\d+)\.')

BUILDER_BASE_IMAGE = "ubuntu:xenial"

DEBUG_OPTIONS = [
    "enable_nacl=false",
    "is_debug=true",
    "is_asan=true",
    "v8_enable_debugging_features=true",
    "v8_enable_object_print=true",
    "v8_optimized_debug=false",
    "v8_enable_backtrace=true",
    "v8_postmortem_support=true",
]

RELEASE_OPTIONS = [
    "enable_nacl=false",
    "is_debug=false",
    "is_official_build=true",
    "use_thin_lto=false",   # crazy disk space usage
    "is_cfi=false", # required when thin_lto is off
    "enable_linux_installer=true",
]

MAGIC_TARGETS = {
    '@std': ['chrome', 'chrome/installer/linux:stable_deb', 'v8_shell', 'v8/test/unittests'],
    '@chrome': ['chrome', 'chrome/installer/linux:stable_deb'],
    '@v8': ['v8_shell', 'v8/test/unittests'],
}

BASELINE_ARTIFACTS = [
    'v8_shell', 'unittests', 'icudtl.dat', 'natives_blob.bin', 'snapshot_blob.bin',
]


def docker_test():
    '''Check to see if Docker is available/compatible with our needs.
    '''
    raw = subprocess.check_output(['docker', '--version'])
    txt = raw.decode('utf8').strip()
    logging.debug(txt)
    M = RE_DOCKER_MAJOR_VERSION.match(txt)
    if not M:
        raise ValueError("Docker version output ('{0}') unparsable".format(txt))
    
    version = int(M.group(1))
    if version < DOCKER_MAJOR_VERSION:
        raise ValueError("Docker major version ({0}) must be >= {1}".format(version, DOCKER_MAJOR_VERSION))


def docker_check_image(image_name: str) -> bool:
    '''Return True if <image_name> exists as a local Docker container image.
    '''
    # Should always succeed (if docker is there and the version test passed)
    raw = subprocess.check_output(['docker', 'image', 'ls', '-q', image_name])

    # Empty output means no such image
    logging.debug("docker image ls '{0}' -> '{1}'".format(image_name, raw.decode('utf8').strip()))
    return bool(raw)


def docker_run_builder(cname: str, iname: str, entry: str, *args, setup_mount: str = None, work_mount: str = None, setup_var="SETUP", work_var="WORKSPACE", privileged=False):
    '''Utility helper to run our builder containers without redundancy.
    Runs the container in the foreground and without automatic cleanup (i.e., no '-d' or '--rm' flags).
    Raises an exception if the docker client executable returns a non-0 exit status.

    Names the container instance <cname>.
    Runs the <iname> image.
    Sets CWD to `{{dirname(<entry>) or '/'}}` and the entry point to `./{{basename(<entry>)}}`
    Passes on any/all <args>.
    If <work_mount> is provided, bind "/work" to <work_mount> and set the ENV var "<work_var>=/work".
    If <setup_mount> is provided, bind "/setup" to <setup_mount> and set the ENV var "<setup_var>=/setup".
    If <privileged> is True, pass "--privileged" as a docker option (helpful for working around SELinux issues)
    '''
    cwd = os.path.dirname(entry) or '/'
    ep = './' + os.path.basename(entry)
    
    docker_args = [
        'docker', 'run',
        '--name', cname]
    
    if work_mount:
        docker_args += [
            '-v', '%s:/work' % os.path.realpath(work_mount),
            '-e', '%s=/work' % work_var]

    if setup_mount:
        docker_args += [
            '-v', '%s:/setup:ro' % os.path.realpath(setup_mount),
            '-e', '%s=/setup' % setup_var]

    if privileged:
        docker_args.append('--privileged')

    docker_args += [
        '-w', cwd,
        iname,
        ep]

    docker_args += args
    logging.debug(docker_args)
    subprocess.check_call(docker_args)


def get_builder_image(args) -> str:
    '''Get the full name/tag of the 'chrome-builder-base' container image to use for checkout/build activity.

    If no such image exists locally, build it.
    
    <args.commit> (a Chromium project commit hash) becomes the per-version tag for chrome-builder.
    '''
    # Check to see if the per-commit image already exists (easy win there)
    commit = args.commit
    if RE_COMMIT_HASHISH.match(commit):
        tagged_name = 'chrome-builder:{0}'.format(commit)
        if docker_check_image(tagged_name):
            return tagged_name
    else:
        tagged_name = 'chrome-builder:latest'
        commit = "origin/master"
        logging.warning("Invalid commit hash '{0}'; provisioning {1} from '{2}'".format(args.commit, tagged_name, commit))

    # Build a builder-image, using <commit> to guide the tool installation process and tagging as <tagged_name>
    #----------------------------------------------------------------------------------------------------------
    
    # Find our setup scripts here
    setup_dir = os.path.realpath(os.path.join(RESOURCE_DIR, "_provision"))

    # Spin up the base image running the given scripts
    cname = "builder-tool-provision-{0}".format(os.getpid())
    logging.info("Running _provision/entry.sh inside image '{0}' (name {1})".format(BUILDER_BASE_IMAGE, cname))
    docker_run_builder(cname, BUILDER_BASE_IMAGE, "/setup/entry.sh", commit, setup_mount=setup_dir, work_mount=args.root, privileged=args.privileged_docker_run)

    # If all of the above succeeded, it's time to COMMIT that resulting image
    logging.info("Committing state of {0} as '{1}'".format(cname, tagged_name))
    subprocess.check_call([
        'docker', 'commit',
        '-m', "Auto-provision of chrome-builder container image for commit '{0}'".format(commit),
        '-c', "WORKDIR /work",
        '-c', "CMD bash",
        cname,
        tagged_name])
    
    # Clean up the dangling container itself
    logging.info("Deleting container {0}".format(cname))
    subprocess.check_call(['docker', 'rm', cname])

    return tagged_name


def do_checkout(args):
    '''Sync (or checkout, if nonexistent) a Chromium source tree to a particular commit hash/version number.
    '''
    # We need Docker available
    docker_test()

    # Get the name/tag of the appropriate builder container (yes, we need that just to checkout)
    builder_image = get_builder_image(args)
    logging.info("Using build image '{0}' to checkout '{1}' to {2}".format(builder_image, args.commit, args.root))

    # Find our setup scripts here
    setup_dir = os.path.realpath(os.path.join(RESOURCE_DIR, "checkout"))

    # Run the builder container, passing in the checkout entry script as the entry point (all the real work happens here)
    cname = "builder-tool-checkout-{0}".format(os.getpid())
    docker_run_builder(cname, builder_image, 
                       "/setup/entry.sh", args.commit, 
                       work_mount=args.root, 
                       setup_mount=setup_dir,
                       privileged=args.privileged_docker_run)

    # Clean up the dangling container itself (no need to snapshot first)
    logging.info("Deleting container {0}".format(cname))
    subprocess.check_call(['docker', 'rm', cname])


def do_shell(args):
    '''Launch the builder container, with the root workspace mounted, and spawn a shell (REPLACES current process).
    
    Requires at least a `tool.py checkout` first.
    '''
    # We need Docker available
    docker_test()

    # Look up the current HEAD of the root git checkout (<args.root>/src/.git/HEAD)
    # (this lets us look up a per-commit builder container image)
    head_path = os.path.join(args.root, "src", ".git", "HEAD")
    if os.path.isfile(head_path):
        with open(head_path, "r", encoding="utf8") as fd:
            args.commit = fd.read().strip()
    else:
        logging.error("Not a valid Chromium source tree (no such file '{0}')! Do you need to `checkout` first?".format(head_path))
        sys.exit(1)

    # Get the name/tag of the appropriate builder container (which actually makes sense, for building)
    builder_image = get_builder_image(args)
    logging.info("Using build image '{0}' to launch shell inside {1}".format(builder_image, args.root))
    
    # Find our setup scripts here (so we can test them manually on target)
    setup_dir = os.path.realpath(os.path.join(RESOURCE_DIR, "build"))

    # Run the builder container with bash as the entry point 
    cname = "builder-tool-shell-{0}".format(os.getpid())
    docker_args = ['docker', 'run', '--rm', '-it']

    if args.privileged_docker_run:
        docker_args.append('--privileged')

    docker_args += [
        '--name', cname,
        '-v', '%s:/work' % os.path.realpath(args.root),
        '-v', '%s:/setup:ro' % setup_dir,
        '-e', 'WORKSPACE=/work',
        '-e', 'SETUP=/setup',
        '-u' 'builder',
        builder_image, '/bin/bash']
    os.execvp(docker_args[0], docker_args)


def do_build(args):
    '''Configure and build a set of Chromium build targets within the context of a source tree.

    Requires at least a `tool.py checkout` first.
    '''
    # We need Docker available
    docker_test()

    # Look up the current HEAD of the root git checkout (<args.root>/src/.git/HEAD)
    # (this lets us look up a per-commit builder container image)
    head_path = os.path.join(args.root, "src", ".git", "HEAD")
    if os.path.isfile(head_path):
        with open(head_path, "r", encoding="utf8") as fd:
            args.commit = fd.read().strip()
    else:
        logging.error("Not a valid Chromium source tree (no such file '{0}')! Do you need to `checkout` first?".format(head_path))
        sys.exit(1)

    # Get the name/tag of the appropriate builder container (which actually makes sense, for building)
    builder_image = get_builder_image(args)
    logging.info("Using build image '{0}' to build '{1}' inside {2}".format(builder_image, args.commit, args.root))

    # Set up the build/output directory
    build_dir = os.path.realpath(os.path.join(args.root, 'src', args.subdir))
    if not build_dir.startswith(args.root):
        logging.error("Build/output directory ({0}) must be somewhere inside Chromium source tree!".format(args.subdir))
        sys.exit(1)
    os.makedirs(build_dir, exist_ok=True)
    logging.info("Using '{0}' as build/output directory".format(build_dir))

    # Generate the options file (args.gn)
    options = dict(o.split('=') for o in args.options)
    args_gn = os.path.join(build_dir, "args.gn")
    with open(args_gn, "w", encoding="utf8") as fd:
        for key, value in options.items():
            if value:
                print("{0}={1}".format(key, value), file=fd)
    logging.info("Configuration placed in {0}".format(args_gn))

    # Expand magic targets (if any)
    targets = []
    for t in args.targets:
        if t in MAGIC_TARGETS:
            targets += MAGIC_TARGETS[t]
        elif t.startswith('@'):
            logging.warning("Unknown magic target '{0}'; ninja will probably not like this!".format(t))
            targets.append(t)
        else:
            targets.append(t)
    logging.info("Build targets: {}".format(targets))

    # Optionally halt here...
    if args.dryrun:
        logging.info("Dry run--stopping here...")
        sys.exit(0)

    # Find our setup scripts here
    setup_dir = os.path.realpath(os.path.join(RESOURCE_DIR, "build"))

    # Run the builder container entry script with the clean-flag/directory/targets
    cname = "builder-tool-build-{0}".format(os.getpid())
    cbuild_dir = os.path.join("/work", os.path.relpath(build_dir, args.root))
    cargs = []
    if args.clean:
        cargs.append("--clean")
    if args.idl:
        cargs.append("--idl")
    cargs.append(cbuild_dir)
    cargs += targets
    docker_run_builder(cname, builder_image, 
                       "/setup/entry.sh", *cargs,
                       work_mount=args.root, 
                       setup_mount=setup_dir,
                       privileged=args.privileged_docker_run)

    # Clean up the dangling container itself (no need to snapshot first here)
    logging.info("Deleting container {0}".format(cname))
    subprocess.check_call(['docker', 'rm', cname])


def do_install(args):
    '''Install build artifacts from a Chromium source tree output directory into a Docker container image.
    '''
    # We need Docker available
    docker_test()

    # Find the build directory 
    build_dir = os.path.join(args.root, "src", args.subdir)
    if not os.path.isdir(build_dir):
        logging.error("'{0}' is not a build directory!".format(build_dir))
        sys.exit(1)
    artifacts = [os.path.join(build_dir, f) for f in BASELINE_ARTIFACTS]
    
    # Find out if this will be a deb-based install
    debs = glob.glob(os.path.join(build_dir, "*.deb"))
    if debs:
        debs.sort(key=lambda f: os.path.getmtime(f), reverse=True)
        newest_deb = debs[0]
        artifacts.append(newest_deb)
        package_name = os.path.splitext(os.path.basename(newest_deb))[0]
        setup_dir = os.path.realpath(os.path.join(RESOURCE_DIR, "install_deb"))
    else:
        logging.warning("No .deb files found in build directory; proceeding with no-deb install...")
        package_name = "test_vv8"
        setup_dir = os.path.realpath(os.path.join(RESOURCE_DIR, "install_nodeb"))
    
    target_image = args.tag.replace("{package}", package_name)
    docker_file = os.path.join(setup_dir, "Dockerfile")
    artifacts.append(docker_file)
    for a in args.artifacts:
        artifacts += glob.glob(os.path.join(build_dir, a))
    artifacts = list(set(artifacts))
    logging.debug(artifacts)

    # Create a temp directory in which to dump the Dockerfile and all artifacts
    with tempfile.TemporaryDirectory() as scratch_dir:
        # Dump artifacts
        for f in set(artifacts):
            shutil.copy(f, scratch_dir)

        # Trigger docker build off the scratch-directory contents
        docker_args = [
            'docker', 'build',
            '-t', target_image,
            '--build-arg', "BASE_IMAGE={0}".format(args.base),
            '--build-arg', "ARTIFACT_DIR={0}".format(args.artifact_dest),
            '--build-arg', "PACKAGE_NAME={0}".format(package_name),
            '--build-arg', "RUN_USER={0}".format(args.run_user),
            scratch_dir] 
        logging.debug(docker_args)
        subprocess.check_call(docker_args)
        

def main(argv):
    # Root/global command options
    ap = argparse.ArgumentParser(description="VisibleV8 build tool")
    ap.add_argument('-l', '--log-level', dest="log_level", metavar="LEVEL", choices=[
        "DEBUG", "INFO", "WARNING", "ERROR"
    ], default="INFO", help="Set logging level to LEVEL")
    ap.add_argument('-d', '--directory', dest="root", metavar="PATH", 
                    default=os.getcwd(),
                    help="Work on (or create) a Chromium source tree rooted inside PATH")
    ap.add_argument('-x', '--privileged-docker-run', dest="privileged_docker_run", action="store_true", default=False,
                    help="Pass '--privileged' to docker-run (i.e., work around SELinux infelicities)")
    ap.set_defaults(handler=None)
    subs = ap.add_subparsers()

    # "checkout" command 
    p_checkout = subs.add_parser('checkout', aliases=['co'], 
                                 help="Check out/sync up a Chromium source tree to a particular commit/version")
    p_checkout.add_argument('commit', 
                            metavar="HASH",
                            help="Chromium project commit hash")
    p_checkout.set_defaults(handler=do_checkout)

    # "shell" command
    p_shell = subs.add_parser('shell', aliases=['sh'],
                                      help="Launch /bin/bash inside the builder container with the workspace mounted")
    p_shell.set_defaults(handler=do_shell)

    # "build" command
    p_build = subs.add_parser('build', aliases=['b'], 
                              help="Configure and build selected targets in a Chromium source tree")
    p_build.set_defaults(options=RELEASE_OPTIONS)
    p_build.add_argument('-s', '--sub-directory', metavar="PATH", dest="subdir", default='out/Builder',
                         help="Perform build inside PATH (relative to Chromium project root)")
    p_build.add_argument('-o', '--option', metavar='NAME=VALUE', dest='options', action='append',
                         help="Add NAME=VALUE to the list of build options")
    p_build.add_argument('--debug', dest='options', action='store_const', const=DEBUG_OPTIONS,
                         help="Use standard debug-build options")
    p_build.add_argument('--release', dest='options', action='store_const', const=RELEASE_OPTIONS,
                         help="Use standard release-build options [the default]")
    p_build.add_argument('--dry-run', dest='dryrun', action='store_true', default=False,
                         help="Stop before launching build (leaving configuration/etc in place)")
    p_build.add_argument('-c', '--clean', dest='clean', action='store_true', default=False,
                         help="Clean before building (default: False)")
    p_build.add_argument('-i', '--idl', dest='idl', action='store_true', default=False,
                         help="Parse Chromium's WebIDL data to produce an 'idldata.json' dump")
    p_build.add_argument('targets', metavar="TARGET", nargs="+",
                         help="One or more TARGETs to build in the Chromium project")
    p_build.set_defaults(handler=do_build)
   
    # "install" command
    p_install = subs.add_parser('install', aliases=['in'], 
                                help="Install Chromium build artifacts into a Docker container image")
    p_install.add_argument('-s', '--sub-directory', metavar="PATH", dest="subdir", default='out/Builder',
                         help="Pull artifacts from build directory PATH inside Chrome source tree")
    p_install.add_argument('-b', '--base', metavar="DOCKER_IMAGE", default="node:lts-jessie",
                           help="Use DOCKER_IMAGE as the base of the new image")
    p_install.add_argument('--artifact-dest', metavar="PATH", dest="artifact_dest", default="/artifacts",
                           help="Place all copied artifacts in PATH inside the new image (default: /artifacts)")
    p_install.add_argument('-t', '--tag', metavar="IMAGE:TAG", default="{package}:latest",
                           help='''\
Name the output image IMAGE:TAG
(default "{package}:latest"; the string "{package}" is 
replaced with the base name of the installed DEB package)
''')
    p_install.add_argument('-u', '--run-user', metavar="USER", default="node",
                           help="Run container entry point as USER")
    p_install.add_argument('artifacts', metavar="GLOB", nargs='*',
                           help="Copy additional artifacts matched by GLOB from build directory to installed image")
    p_install.set_defaults(handler=do_install)

    # MAIN ENTRY LOGIC:
    ###################
    args = ap.parse_args(argv[1:])

    args.root = os.path.realpath(args.root)
    assert os.path.isdir(args.root)

    logging.basicConfig(level=getattr(logging, args.log_level))
    if args.handler:
        args.handler(args)
    else:
        ap.print_help()


if __name__ == "__main__":
    main(sys.argv)

