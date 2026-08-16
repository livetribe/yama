# ADR-014: A Run Generates One Package at a Time

## Status

Proposed

## Context

A Yama run covers several packages. The command takes Google Wire's own
package-pattern argument, so `./...` is an ordinary invocation (ADR-012).

A run moves files aside while it works. It moves Google Wire's output name aside,
so that a `wire_gen.go` that the application owns survives generation (ADR-008).
It moves the committed lifecycle file aside as well. That file and the lifecycle
placeholder declare the same constructors, and Google Wire and Yama both
type-check the package (ADR-011).

Two questions follow from those two facts, and no document answers either one.

The first question is what a failure in one package does to the others. Google
Wire commits the packages that generated and reports the packages that did not.
ADR-012 binds Yama to Google Wire's observable behaviour, so Yama must do the
same. A run that ends on the first failure regenerates nothing, which is a
different observable behaviour.

The second question is which unit puts a package's files back. Every package in a
run holds its own two files. A run can end in a different way for each package,
so it needs one answer for each package. It must give that answer even when a
sibling package failed earlier.

A third fact makes the two questions interact. It applies only to a run that
continues past a failure. Yama's load over Google Wire's output type-checks the
whole package, and it resolves an import of a sibling package from source. A
package that Google Wire rejected declares no constructors at all while its
lifecycle file is aside, for three reasons:

* a build tag that the load does not set guards the lifecycle stub;
* Google Wire wrote no output for the package;
* the committed lifecycle file is at the backup name.

A run that continues would therefore fail to type-check an importer of that
package. The diagnostic would name the importer rather than the package that
Google Wire rejected. A run that ends at the first failure never reaches that
load. This hazard therefore belongs to the decision below, and not to the
behaviour that Yama has today.

## Decision

### A run is three phases over one work item for each target package

Yama creates one work item for each target package. It then runs three phases,
and each phase visits every work item once. The phases are `Prepare`, `Generate`,
and `Complete`. Yama invokes Google Wire once, between `Prepare` and `Generate`.

`Prepare` reads the package's lifecycle stubs and moves its two files aside.
`Generate` reads Google Wire's output, computes the levels, and writes the
lifecycle file. `Complete` puts the package's files back, or drops the backup
that the emitted file replaced.

### A work item's type states its outcome

`Prepare` and `Generate` each return a work item. Each phase returns an item of a
different type for a package that failed. That type declines the phases that
follow, and it declares the settlement that its own situation needs.

A work item carries no phase number, no error flag, and no predicate that reports
whether the run is still generating for that package.

### A failure in one package does not stop another package

Yama reports a failure through the work item that holds it. Every phase visits
every item, so a package that failed settles its own files, and a package that
succeeded still emits its lifecycle file. The run reports a failure for each
package that failed, and it exits with a non-zero code.

This holds for every failure that belongs to one package:

* a lifecycle stub that does not load;
* a file that Yama cannot move aside;
* an injector shape that Yama cannot parse;
* a package that Google Wire rejects.

### The work item that moved a file is the work item that puts it back

A work item records the names that it moved aside. Its `Complete` acts on that
record, and on no other name. A work item that moved no file puts no file back,
whatever else happened in the run.

### A package moves its lifecycle file aside before Google Wire's output name

A failure to move either file leaves the package out of the Google Wire
invocation. The order decides which file a failed move leaves at the backup name.

### A package that Google Wire rejected settles like every other package

Yama puts the files of a rejected package back in `Complete`. It puts them back
together with the files of every other package. No step runs between Google Wire
and the generate phase.

A package that imports a rejected package therefore fails to type-check during
the run. The run already reports a failure. The importer also keeps every file
that it held when the run started.

### A package states its own outcome from what its directory holds

The generate phase reads the target directory first. It finds no Google Wire
output for a rejected package. The work item then moves to a state that writes
no file and returns no error.

Two different packages reach that state. Google Wire rejected the first. The
second declares no lifecycle stub, so Google Wire had nothing to generate for
it. What the directory holds does not say which package it is. Neither package
needs a count of its stubs, because neither one reports the outcome of the run.

### An error signals that a run failed, and carries nothing else

An error coordinates behaviour. Nothing reads a field out of an error. Nothing
prints an error for a person to read. A caller maps a non-nil error to the exit
code.

Two things produce an error. A package produces one for its own failure. The
Google Wire invocation produces one when Google Wire fails at either grain. A run
joins the two. A rejected package therefore sets the exit code through the Google
Wire invocation, and not through its own settlement.

### Custody exports functions and declares no type

One package owns every move of a file in a target directory. It exports three
functions: one moves a name to the backup name, one moves the backup name back,
and one removes the backup. It holds no state between calls, and it names
nothing that belongs to Yama.

### Each part of a run writes its own messages

A work item writes its own progress line when it commits a file. The Google Wire
invocation writes Google Wire's diagnostic. The error that a run returns states
that the run failed, and a caller reads it for the exit code.

## Rationale

### Google Wire's own behaviour decides the failure grain

ADR-012 binds the Yama command to Google Wire's observable behaviour. It also
states that a measurement settles a question that no ADR answers. Google Wire
commits each package that generated, reports each package that did not, and exits
with a non-zero code. A run that stops at the first failure gives a different
answer to the same input, so it breaks the parity ADR-012 states.

The cost of the parity is one property that a whole-run failure has and a
per-package failure does not. A tree after a per-package failure holds a fresh
lifecycle file for some packages and the previous file for others. ADR-012
accepts that Yama inherits Google Wire's judgements, and this is one of them.

### A type states an outcome more exactly than a field does

A field that records an outcome works only if every phase reads it. A phase that
does not read the field does the wrong work for a package that failed. The
compiler reports nothing about the phase that forgot.

A type states the same outcome once, at the point where the run learns it. The
phases that follow need no guard, because the failed type answers them with
itself. Each type declares the settlement that its own situation needs, and the
compiler holds every type to the whole interface. A reader who asks what a
rejected package does with its files reads one type.

### Ownership of a file follows the record of moving it

A run that fails in several places needs to know which files are where. A record
on the work item answers that question for one package, and the answer does not
depend on what any other package did.

This also bounds what a `Complete` can do. It acts on the names in its own record.
It cannot restore a name that its package never moved, and it cannot leave a name
that its package did move.

### The move order decides which file a failure leaves at the backup name

A move can fail. The move that ran first then holds a file at the backup name.
The Go toolchain does not read a file with a name that starts with a dot.

Yama can write the committed lifecycle file again. The next successful run
produces it. Yama cannot write the application's `wire_gen.go` again, because the
application owns that file and Yama only borrows the name (ADR-008).

A run that moves the lifecycle file first therefore risks only the recoverable
file. A failure to move the lifecycle file leaves the application's `wire_gen.go`
where it is, because Yama did not touch that file yet.

### The importer of a rejected package fails, and that failure costs one message

Yama's load over Google Wire's output type-checks the package. That load resolves
an import of a sibling package from source, so a package that declares nothing
breaks every package that imports it.

A package that Google Wire rejected declares nothing while its lifecycle file is
aside. Its stub is behind a build tag that the load does not set. Google Wire
wrote no output for it. Its committed file is at the backup name. An importer of
that package therefore fails.

That failure adds one message to a run that already fails. Google Wire rejected a
package, so the run exits with a non-zero code whatever the importer does. The
importer settles its own files, and it keeps every committed file that it held.
Google Wire's own diagnostic names the rejected package and points at that
package's stub, so the user reads the real cause first.

A restore before the generate phase removes that one message. The restore cannot
wait for `Complete`. `Complete` runs after every package reads Google Wire's
output. The restore cannot run inside the generate phase either. That phase
visits one package at a time. The package that it visits first reads Google
Wire's output before a later package restores its own file.

A restore therefore needs a step of its own, and a state for the items that the
step converts. That gain does not justify a step and a state.

### A restore is safe only for a package that wrote no lifecycle file

The generate phase writes a fresh lifecycle file for each package that Google
Wire accepted. That file and the committed lifecycle file declare the same
constructors. A restore of the committed file would therefore declare those
constructors twice, and the package would not compile. Such a package drops its
backup instead of restoring it.

A package that Google Wire rejected writes no lifecycle file, and nothing else
declares its constructors. A restore of the committed file therefore adds a
declaration rather than duplicating one. A package that declares no lifecycle
stub also writes no lifecycle file, and it settles the same way.

### Functions state custody's contract more exactly than a handle does

Custody moves a file and reports whether the move succeeded. It holds nothing
between one call and the next. A type would therefore carry no state, and a
handle would carry no fact that the work item does not already record.

Functions also keep the package free of Yama's vocabulary. Each one takes a
directory and a plain filename. Nothing in the package names a stub, an injector,
a package pattern, or a run.

### A message printed where it is produced needs no carrier

A run produces two kinds of message. A work item produces a progress line when it
commits a file. The Google Wire invocation produces Google Wire's diagnostic.

Each of those points already holds the message and already knows the stream. A
value that carried the messages to a caller would add a type whose only work is
to move text. The caller would then print what it received, and add nothing.

## Consequences

### Positive

* A malformed stub in one package no longer denies every other package its
  lifecycle file. A run over `./...` in a large repository regenerates what it
  can.
* The compiler checks the settlement of every failure. A new failure needs a new
  type, and that type does not compile until it declares what to do with its
  files.
* A test can drive one work item through its phases without a Google Wire
  subprocess, because the item holds its own state and its own record.
* Custody is testable on its own, over a temporary directory, with no Yama
  vocabulary in the test.
* A package states its own outcome from what its directory holds. A run therefore
  needs no step between Google Wire and the generate phase, and no count of a
  package's stubs.
* An error stays a signal. No type in the run carries a report that nothing
  prints.

### Negative

* A tree after a failed run is not uniform. Some packages hold a fresh lifecycle
  file, and others hold the file from the previous run. A reader of the tree
  cannot tell which is which without reading the run's output.
* An importer of a package that Google Wire rejected fails to type-check during
  the run, and reports a missing constructor rather than the real cause. The
  importer keeps every file that it held. Google Wire's own diagnostic names the
  rejected package.
* Yama does not put a file back when it stops without running `Complete`. A panic
  therefore leaves two files at their backup names, and the user moves them back
  by hand. Google Wire moves no file, so a panic in Google Wire costs the user
  nothing.
* Two Yama runs over one directory at the same time still corrupt each other.
  ADR-008 records that hazard, and this decision does not remove it.

### Accepted Trade-Off

The project accepts a tree that a failed run leaves in two states, and accepts
that a panic leaves two files at their backup names. In exchange a run reports
every package that failed rather than only the first, and it emits every
lifecycle file that it can.

## Rejected Alternatives

### End the run at the first failure

Rejected. It gives a different observable behaviour from `wire gen` for the same
input, which ADR-012 forbids. It also hides every failure after the first, so a
user fixes one package, runs again, and learns about the next one.

### Record the outcome in a field on one work type

Rejected. Every phase then reads the field before it acts, and a phase that does
not read it does the wrong work for a failed package. The compiler checks none of
those reads. The settlement for each kind of failure also ends up in one function
that branches, rather than beside the failure it settles.

### Restore a rejected package inside the generate phase

Rejected. That phase visits one package at a time. A package that it visits first
reads Google Wire's output before a package that it visits later restores itself.
The restore would therefore help some importers and not others. The order of the
package list would decide which importers it helps.

### Restore a rejected package in a step before the generate phase

Rejected. The step gives every importer the same answer. The alternative above
cannot do this. The step costs a pass over the work items, between Google Wire
and the generate phase. It also costs a state for the items that the pass
converts. It removes one message from a run that already fails, and only for a
package that a previous run generated. A package generated for the first time has
no committed file to restore, so its importer fails in either design.

### Report a rejected package through its own error

Rejected. A directory that holds no Google Wire output belongs to one of two
packages. Google Wire rejected the first. The second declares no lifecycle stub.
What the directory holds does not say which one it is.

An error from that package would therefore fail every stub-free directory in a
repository. A run over `./...` would then report packages that had no work to do.
A count of the package's stubs would tell the two apart, and it would add a count
that nothing else needs.

The Google Wire invocation already knows which packages it rejected. Its error
already sets the exit code.

### Give custody a type that holds an open move

Rejected. Custody holds no state between calls, so the type would hold no state
either. Every fact that such a handle could carry is already in the work item's
record of what it moved. The type would also have to name what it is holding, and
that name belongs to Yama rather than to a package that moves files.

### Restore every package's lifecycle file after Google Wire runs

Rejected. A package that Google Wire generated for holds an output file that
carries the lifecycle placeholder. A restore of the committed file would declare
the same constructors twice, and the package would not compile for every load
that follows.

### Recover from a panic to put the files back

Rejected. A deferred call would put the files back without catching the panic,
and that costs one line. Only a bug in Yama produces the case. The line would
therefore state a hazard that a reader never meets, and Google Wire guards
nothing of its own this way.

### Report the run through one result value

Rejected. The value would carry one entry for each package and a message for the
run. Each entry copies what the work item already holds, and the run's message is
text that the point of production could print directly. Nothing reads the value
except the code that prints it.

## Non-Goals

This decision does not do these things:

* It does not change what Yama emits. The content of `lifecycle_gen.go` is
  settled by ADR-004, ADR-008, ADR-010, and ADR-013, and this decision leaves
  every one of them as it is.
* It does not change how Yama derives ordering or folds a cleanup (ADR-008), and
  it does not change how Yama detects a lifecycle capability (ADR-004).
* It does not change the command's flags, its package-pattern argument, or its
  exit codes (ADR-012).
* It does not change the runtime public API (ADR-007).
* It does not state how a run behaves when two runs cover one directory at the
  same time. ADR-008 records that hazard.
* It does not make Yama recover from a panic, or from a run that an operator
  stops.
