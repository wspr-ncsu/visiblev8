# VisibleV8 Functionality Tests

Automated tests used to check that a new build of VisibleV8 (specifically `v8_shell`)
is capturing the expected events and logging them in the expected format.

## Contents/Overview

* `logs/`: expected results and required tools/scripts
    * `entry.sh`: bootstrap logic for running tests inside container running a vv8 build
    * `relabel.py`: utility Python script that normalizes irrelevant/ephemeral differences between corresponding log files
    * `vlp.py`: internal dependency for `relabel.py` (i.e., slow reference-implementation for parsing VV8 logs)
    * `trace-apis/`: repository of expected-output log files for running the tests under `trace-api` patchsets
* `src/`: test JS files and other required resources
* `run.sh`: master launch script that runs tests inside a containerized build of vv8 (iff that container includes `v8_shell` in the `/artifacts` directory)

## Log Creation

One log file per Chromium thread (iff that thread engages VV8's instrumentation), created in the Chrome process's *current working directory* and named `vv8-$TIMESTAMP-$PID-$TID-$THREAD_NAME.log`.  (Some older patchsets, e.g. for Chrome 64, unfortunately omitted the `$PID` segment of the name...)

Note that single log files may or may not correspond to single page/domain request lifetimes; you'd best have independent sources of data about that, although the `@` context messages in the log stream should help a lot.
(Also, current internal development of VV8 is moving away from `@` context messages for something more useful, namely, Blink-derived execution context IDs that can be tied to Blink frame contexts.)

## Log Format

Text is ASCII encoded (all unprintable ASCII characters are \xNN escaped, all code points above ASCII 127 are \uNNNN escaped).

One record per line.  The first character determines the kind of record.
The rest of the line (characters 1-N) comprise one or more `:` delimited fields.
(`:` characters existing inside the log data are `\` escaped, as are `\` characters appearing in log data).

### Data Types/Formats

V8 and JS values are formatted according to the following rules:

* String values are flanked by double quotes (e.g., `"string"`) but do not escape internal quotes

* Numeric values are printed "as-is" per usual C++ iostream formatting rules (e.g., `42`, `3.1415926`)

* Regular expression objects are printed as `/PATTERN/`, where *PATTERN* is the non-quoted string representation of the regular expression pattern

* JavaScript's delightful "oddball" values are printed in short form:

	* `#F` for `false`

	* `#T` for `true`

	* `#N` for `null`

	* `#U` for `undefined

	* `#?` for any other V8-specific oddball type that leaks into the log data

* Functions are printed as their string names (**not** quoted); anonymouse functions are `<anonymous>`

* Objects are printed as `{Constructor}` where **Constructor** is the name of the function that constructed the object

* V8's internal object representation is complex and full of special cases; if the logging code ever encounters a value it isn't sure about, it logs it as a single `?`


### Record Types/Formats

* `~`: (possibly) new Isolate context (basically, a namespace for all script IDs/etc.)

	* First (only) field: Isolate address (i.e., a per-process-unique opaque identifier)

* `@`: (possibly) new `window.origin` value of current isolate/context

	* First (only) field: either the [quoted] JavaScript string value retrieved from the property, or `?` if that's unavailable

* `$`: script provenance (from HTML, JS file, `eval`, ...)

	* First field: new script's integer ID

	* Second field: either the new script's name/URL (a quoted JS string), or the parent script's script ID (in cases where this script was `eval`'d into existence)

	* Third field: unquoted JS string containing full script source

* `!`: execution context (subsequent log entries running THIS script context)
	
	* First (only) field: active script ID (in the current Isolate script-ID-space)

* `c`: function call
	
	* First field: character offset within script

	* Second field: function object/name
	
	* Third field: receiver (`this` value)

	* Rest of fields: positional arguments to function

* `n`: "construction" function call (e.g., `new Foo(1, 2, 3)`)

	* First field: character offset within script

	* Second field: function object/name

	* Rest of fields: positional arguments to function

* `g`: property value get

	* First field: character offset within script
	
	* Second field: owning object

	* Third field: property name/index

* `s`: property value set

	* First field: character offset within script

	* Second field: owning object

	* Third field: property name/index

	* Fourth field: new value (arbitrary type)


