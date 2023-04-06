#!/bin/sh

PACKAGE_NAME=`find /artifacts -name "vv8-shell*" -printf "%f\n"  | sort -V | tail -n 1`
VERSION=`echo $PACKAGE_NAME | grep -o -E '[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*'`
V8_SHELL="/artifacts/$VERSION/vv8-shell-$VERSION"
UNITTESTS="/artifacts/$VERSION/unittests"

WORKSPACE="/work"
TOOLS="/tools"
TEST_SRC="/testsrc"
EXPECTED_LOGS="/expected"
SCRATCH_DIR=$(mktemp -d)

# V8 unit tests still don't run
# if [ -x "$UNITTESTS" ]; then
#     echo "Running V8's unittests..."
#     if ! "$UNITTESTS" --gtest_filter=-SequentialUnmapperTest --gtest_output=xml:"v8_unittests_results.xml" >/dev/null 2>&1; then
#         if [ -r v8_unittests_results.xml ]; then
#             failures=$(grep "^<testsuites " v8_unittests_results.xml | grep -oP 'failures="\K\d+')
#             echo '*****************************************************************'
#             echo "WARNING: V8's unittest suite reported $failures failures!"
#             echo "         v8_unittests_results.xml should be MANUALLY INSPECTED..."
#             echo '*****************************************************************'
#             echo
#         else
#             echo '*****************************************************************'
#             echo "WARNING: V8's unittest suite did not produce an output file!"
#             echo "         you may want to run it manually to observe the output..."
#             echo '*****************************************************************'
#             echo
#         fi
#     fi
# else
#     echo "No V8 unittests found (at $UNITTESTS); skipping unit tests..."
# fi

exitstatus=0
if [ -x "$V8_SHELL" ]; then
    echo "Running VisibleV8 output tests..."
    for script in $TEST_SRC/*.js; do
        sbase=$(basename "$script")
        sbase=${sbase%.js}

        echo -n "  $script: "
        "$V8_SHELL" "$script" >/dev/null

        expected="$EXPECTED_LOGS/$sbase.log"
        actual="$SCRATCH_DIR/$sbase.actual.log"
        mv vv8-*-vv8-shell-*.0.log "$actual"
        
        "$TOOLS/relabel.py" <"$actual" >"$SCRATCH_DIR/filtered_actual.log"
        "$TOOLS/relabel.py" <"$expected" >"$SCRATCH_DIR/filtered_expected.log"
        if DIFFS=$(diff "$SCRATCH_DIR/filtered_actual.log" "$SCRATCH_DIR/filtered_expected.log"); then
            echo "OK"
        else
            echo "FAIL"
            echo "-----------------------"
            echo "$DIFFS"
            echo "-----------------------"
            exitstatus=1
        fi
    done
else
    echo "No v8_shell found (at $V8_SHELL); skipping output-suite tests..."
fi

exit $exitstatus

