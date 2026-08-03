# ADR-013: Derived Identifiers in Generated Code

## Status

Proposed

## Context

Yama does not choose most identifiers in an emitted `lifecycle_gen.go`. The
application names each lifecycle constructor when it writes the stub (ADR-011).
Each component local keeps the name Google Wire gave it, because Yama re-emits
Wire's construction body unchanged (ADR-008).

Yama derives two identifiers. It derives a name for each cleanup local, because
Wire names a cleanup positionally and the re-emitted body hands each cleanup to
a different level (ADR-008). It derives a name for each derived injector, in a
reserved namespace, so that one stub maps to one injector in Wire's output
(ADR-011).

ADR-008 states what a derived identifier must be. It must be deterministic
across equivalent inputs. It must be stable enough to review across
regenerations. It must be unique in the generated package. Neither ADR states a
rule that meets those three obligations. This ADR states that rule.

The emitted file also carries import names. Yama chooses those names. A component
local shares one scope with every import name in the file. Yama does not choose
component locals, so a component local can carry the name Yama wants for an
import. Nothing so far says which name gives way.

## Decision

Yama derives every identifier it owns by the rules below.

### A cleanup local is named for the value it releases

The name is the value's own local name, followed by `Cleanup`. A cleanup
returned beside `base2` is bound to `base2Cleanup`.

### A derived injector is named `yama_` followed by the stub's name

A stub named `NewLifecycle` produces an injector named `yama_NewLifecycle`.
Yama recovers the stub's name by removing the prefix.

Identifiers that start with `yama_` are reserved. An application must not
declare a Wire injector whose name starts with `yama_`.

### The emitted file imports Yama under the names `yama` and `rt`

`yama` is the public package. `rt` is the runtime-support package (ADR-010).
Yama writes an explicit import alias when the import path does not already give
the package that name.

Google Wire's output sometimes imports one of those two paths already, because a
provider's signature names a type from it. Yama then refers to the package by
the name that import carries, and adds no second import of one path.

### A taken name is escaped with the lowest free number

When a name this ADR derives is already taken in the scope where it must be
unique, Yama appends a decimal number. It uses the lowest number, starting
at 2, that makes the name free.

Two scopes apply, and they hold different names.

A cleanup local must be unique in its injector's function scope. That scope is
the injector's parameters, its component locals, and its other cleanup locals.

An import name must be unique in the emitted file's scope. That scope is:

* every identifier in the re-emitted bodies,
* every constructor parameter name, the options parameter included,
* every constructor name,
* every identifier that the application declares in the package block,
* every predeclared identifier,
* every import name that the file already carries.

The package block is in that list because the emitted file shares it with the
application. Go permits no identifier in both a file block and the package block. An
application that declares `rt` in any file of the package makes that name
unusable for an import in the emitted file.

### Three kinds of name give way in a fixed order

A collision has more than one name that could move, so this ADR fixes which one
does.

1. **An import the re-emitted body refers to does not move.** The constructor
   reproduces that body unchanged (ADR-008), and the body names the package. A
   rename would have to rewrite the re-emission.
2. **The options parameter gives way to those imports.** A parameter can
   carry the name of a package that the body refers to. That parameter shadows
   the package inside the body. No import rename frees the name, because the
   parameter and the reference are in one scope. Yama renames the parameter. A
   parameter is positional, so no caller sees the change.
3. **Yama's own two imports give way to everything else.** They are the last
   names chosen and the only ones with no other claim on them.

### A constructor name is reported, not escaped

An application calls its lifecycle constructor by name, so Yama cannot rename
one. A name the package already declares is therefore a failure and not an
escape.

Yama adds no check for it. It loads the stubs with type checking, and that load
is the only one that sees the stubs beside the application's other files. The Go
compiler reports the redeclaration there.

## Rationale

### The value's name is the only stable fact about a cleanup

Wire numbers cleanups in the order their providers run. That number describes
the injector body and nothing else. Inserting one provider ahead of another
renumbers every cleanup after it, and the re-emitted body then hands
differently-named cleanups to the same levels. Naming the cleanup for its value
removes that coupling. The name then changes only when the value's own name
changes.

### An underscore makes the reservation a rule a person can apply

`yama_` is one token. A reader can test any injector name against it without
knowing how Yama builds the rest of the name. Without the separator, the
prefixed form of a stub named `newLifecycle` is `yamanewLifecycle`, which does
not show where the prefix ends. Google Wire also writes an underscore into its
own generated names, such as `_wireFileValue`, so the character already carries
this meaning in the same file.

### Renaming a component local would make the re-emission unfaithful

ADR-008 requires the constructor to reproduce Wire's construction body. A
component local can appear many times in that body. Renaming one means
rewriting every reference to it, and the result no longer matches the injector
a reviewer compares it against. An import name appears only in the constructor's
tail, where Yama writes every reference itself. The import is therefore the
cheaper name to change, and changing it leaves the re-emission untouched.

### A numeric suffix is the convention already present in the file

Wire disambiguates its own repeated names with a decimal suffix, as in `cleanup`
and `cleanup2`. Yama's escape produces the same shape. The emitted file
therefore carries one disambiguation convention rather than two.

## Consequences

### Positive

Every identifier in the emitted file has a stated source. It is the
application's, or Wire's, or derived by one of the rules above. A reviewer
can predict the whole file from the stub and Wire's output.

A committed `lifecycle_gen.go` changes only when the application's own names
change, or when Yama's analysis changes. The derivation contributes no churn of
its own.

### Negative

The reserved prefix constrains the application. An application can declare its
own Wire injector named `yama_NewLifecycle`. If that injector sits in a package
that also holds a stub named `NewLifecycle`, the package gets two declarations
of one name. The Go compiler reports that as a redeclaration when Yama runs
Wire.

Any identifier in the emitted file's scope can force an escaped import name.
A component local named `rt`, a package-level `rt` in another file that the
application wrote, or an options parameter named `rt` each produce the import
name `rt2`. The emitted file stays correct. The escape stays deterministic.
But that constructor reads worse than one with the plain names.

An application that declares `yama` in the package block must also alias its
own import of the public package, in the stub file that it writes by hand.
Yama escapes the names in the file that it emits. It does not edit the stub.

### Accepted Trade-Off

Yama reads the package block as one build configuration loads it. A
declaration behind a build tag that load did not carry is not in the scope
that Yama checks. The emitted file can then fail to build under that tag.
Google Wire reads its own scope the same way and has the same limitation.

A cleanup local's name depends on a name Wire chose for the value. Renaming a
provider in the application therefore changes the emitted file, and the change
reaches beyond the line the rename touched. Yama accepts that coupling. The
alternative is to keep Wire's positional names, which ADR-008 rejects for the
re-emitted body.

## Rejected Alternatives

### Rename the colliding component local instead of the import

This keeps the import names `yama` and `rt` in every emitted file, but it
requires rewriting every reference to the renamed local. The rewrite is
mechanical. Its output no longer matches the injector body that Wire produced.
That body is the one artifact a reviewer can check the re-emission against.
Yama would also have to prove that it rewrote every reference, including the
ones inside struct literals and error guards. Renaming the import needs no
such proof.

### Import one path twice, under two names

A body that refers to a package as `rt` could keep that import. Yama could add
a second import of the runtime-support package under another name. This does
not work. The parameter that shadows `rt` shadows it for every reference in
that body, so the body's own import is still unreachable.

It also costs the emitted file its byte stability. The import block is ordered
by path, which is a total order only while one path has one name.

### Prefix the derived injector name with no separator

`yamaNewLifecycle` is shorter and reads as one Go identifier. It also produces
`yamanewLifecycle` for a stub whose name starts with a lowercase letter, and it
gives a reader no way to see where the prefix ends. The reservation then has to
be stated as a length rather than as a token.

## Non-Goals

This ADR does not name a lifecycle constructor. The application names it in
the stub (ADR-011). It does name that constructor's options parameter, but
only when the application's choice collides with a package that the body
refers to. A parameter name reaches no caller, so this is not a name that the
application publishes.

This ADR does not rename component locals. They keep the names Wire gave them
(ADR-008).

This ADR does not cover identifiers inside the runtime-support package. Those
are that package's own private names (ADR-010).

This ADR does not decide which levels a constructor declares, or which member
form each level member takes. That is lifecycle analysis, not naming.
