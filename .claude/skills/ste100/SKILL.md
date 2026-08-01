---
name: ste100
description: Write or convert prose to ASD-STE100 (Simplified Technical English) using a concrete rule checklist, and verify a conversion against its original for meaning drift, not just style. Use whenever asked to write, rewrite, simplify, or check documentation — ADRs, technical docs, comments, or anything else — in "ASD-STE100", "STE", "Simplified Technical English", or "plain English" style, or to verify STE conformance. Use proactively before treating any STE-style rewrite as done, even if not asked to check it.
---

# ASD-STE100 (Simplified Technical English)

"Write it simply" is not enough instruction to produce STE reliably — general
prose habits (em dashes, semicolons, nested subordination, metaphor) are
strongly trained in, and vague guidance to simplify either overcorrects into
choppy, meaningless fragments or lets those habits back in without the writer
noticing. Run the checklist below explicitly, sentence by sentence, rather
than relying on a general sense of "simple."

## Two situations, two risk profiles

**Drafting new prose in STE** is lower risk: apply the checklist as you
write, self-check before calling a passage done.

**Converting existing prose to STE** is higher risk, because the actual goal
is not just STE-conformant text — it is text that means exactly what the
original meant. These are independent failure modes: a rewrite can satisfy
every rule below and still assert something the original didn't, or drop the
clause that gave a sentence its point while leaving it perfectly grammatical.

For conversion, always run two separate passes:

1. **Draft** the rewrite, sentence by sentence.
2. **Verify** — reread the rewrite against the original, sentence by
   sentence, checking specifically for meaning drift (see "Verification
   pass" below), as a distinct step from applying the checklist. Do this
   pass yourself if nothing else is available, but know it is weaker than an
   independent check: you already believe the rewrite says what you meant it
   to, which is exactly the blind spot that lets drift through. For a
   document that matters, hand the original and the rewrite to a fresh
   subagent with no visibility into how the draft was produced, and have it
   check only for meaning drift. It has no reason to trust the rewrite.

## The checklist

Apply this to every sentence in scope.

### 1. One thought per sentence

Don't chain independent clauses with semicolons, colons, or "and"/"but"
across a clause boundary. A colon introduces a genuine list of items —
never another full clause.

Bad: "This left a contradiction that nobody noticed: X's Y skips Z in
silence, matching W — its doc comment states this, and a test asserts it."
(one sentence, four assertions, joined by colon + em dash + "and")

Good: "This left a contradiction that nobody noticed. X's Y skips Z in
silence, matching W. Its doc comment states this. A test asserts it."

### 2. No em dashes as clause connectors

An em dash joining two clauses is rule 1's violation wearing different
punctuation. Split into separate sentences instead.

### 3. No metaphor or idiom — literal language only

STE readers may not be native English speakers; figurative language doesn't
translate. If a sentence uses a spatial or physical image for an abstract
relationship, restate it literally.

Bad: "It splits the rule along a line nothing defines."
Good: "It has two rules. Nothing says which one applies to a new question."

Bad: "which side a new question falls on"
Good: "which rule covers a new question"

### 4. Keep relative pronouns explicit, and don't stack them

Don't elide "that"/"which," and avoid nesting more than one relative clause
in a sentence — each nested clause is a place meaning can get lost.

Bad: "a second command-line vocabulary that a Wire user must learn to use a
tool built on the assumption that they already know Wire" (four levels of
embedding)

Good: "Answering each question on taste builds a second vocabulary. A Wire
user must learn that vocabulary. The tool assumes they already know Wire."

### 5. Target roughly 20-25 words per sentence

A longer sentence is where the other violations hide. Treat length as a
signal to look for a chainable clause, not just a place to add a comma.

### 6. Active voice, real subjects — not a robotically repeated one

Prefer a concrete actor as the subject (a named system, file, or document)
over passive constructions. But don't overcorrect into using the same
generic filler subject ("A person...," "The project...") for every sentence
— that reads as robotic and repetitive, which is its own real failure, not
a virtue of "being active." Vary the subject naturally with whatever is
actually acting in that sentence.

### 7. When splitting a sentence, check the pieces still mean something

The most common way meaning is lost during simplification: a long original
sentence is cut at a clause boundary, and the kept piece doesn't carry the
clause that gave it its point.

Bad: "Generation is one go:generate directive." (dropped "...that invokes
Yama, which runs Wire" — the clause that made the sentence assert anything)

Good: "A project needs one go:generate directive to generate. That
directive invokes Yama, and Yama runs Wire."

After splitting any sentence, reread the new first piece alone: does it
assert a real, complete claim, or does it only parse grammatically?

### 8. Preserve the exact scope of negations and quantifiers

"Not every X" (existential — some X are an exception) is a different claim
from "not always" or "doesn't want every" (which stack ambiguously and can
read as universal, existential, or habitual depending on the reader). When a
sentence has a negation plus a quantifier, restate the same logical shape in
different words — don't reconstruct it from a general sense of what it meant.

Bad: original "not every injector's graph is one an application wants
orchestrated" (some injectors are unwanted) rewritten as "does not always
want to orchestrate every graph" (three readings, none clearly the original)

Good: "Some injectors build a graph the application doesn't want
orchestrated."

### 9. Preserve modal strength

Don't silently drop or soften words like "necessarily," "always,"
"overwhelming majority," and don't swap a duration connector ("for as long
as X") for an event connector ("until X") — these look like simplifications
but change what's being claimed.

Real example of the swap breaking logic: "...ran both at once **for as long
as** nobody compared them" (a state that persists while a condition holds)
became "...ran both at once **until** nobody compared them" (an event that
ends the state) — but "nobody compared them" was true from the start and
never became newly true, so "until" has nothing to trigger on.

### 10. One word per concept, held consistently

Don't let short-sentence pressure collapse distinct concepts onto one word.
If a project already distinguishes terms — "emit" for what a generator
produces, "generate" for what an upstream tool produces, "write" for what a
person authors by hand, say — keep the distinction. Search the codebase or
docs for the term before assuming it's free to reuse or reword; a term that
feels generic is often established somewhere specific, and a grep settles it
in seconds where a guess doesn't. Don't rely on a general sense of what's
"probably" already established — check.

## After the checklist, read it again as a reader

A checklist finds what it names, in the order it names it, and it can make
you stop looking once you've been through the list — including for things
the checklist itself covers. In one test run, an agent worked through this
exact checklist against a flawed passage and still missed the passive-voice
violation in rule 6, a rule that was right there in the list it had just
applied. Going through numbered items one at a time is not the same activity
as reading the passage.

So after finishing the numbered pass — whether you're drafting, converting,
or verifying someone else's rewrite — read the whole passage straight
through once more, as an actual reader would, with the checklist out of
mind. Ask only: does anything here read badly, sound vague, feel
overcomplicated, drop something specific for something general, or
contradict itself anywhere? Anything that snags is worth tracing back to
whichever rule it violates — there is almost always one — even if that
sentence already passed the numbered pass. The checklist is what tells you
*why* something is wrong once you've noticed it; it is not a substitute for
noticing.

This matters most in verification mode, where the temptation is to treat a
completed run through the ten rules as the whole job. It isn't — it's the
mechanical half.

## Verification pass (conversion mode)

Read the original and the rewrite side by side, sentence by sentence (or
clause by clause, where sentences were split or merged). For each pairing,
ask one question: **does the rewrite assert the same thing the original
did — no more, no less, no different scope?**

This is a different question from "is the rewrite STE-conformant" — a
sentence can pass every rule above and still fail this one. Flag:

- A claim in the rewrite that's stronger, weaker, or differently scoped than
  the original.
- A grammatical-but-vacuous sentence (parses fine, asserts nothing real —
  see rule 7).
- A dropped qualifier, example, or fact that changes what a reader would
  conclude.
- Added content not present in the original, even if it reads as a
  reasonable inference.

## Two more checks, same review pass

Not STE rules, but caught the same way — a careful read against a stated
reference — so worth running in the same pass:

**Self-consistency against stated scope.** If the document has a section
stating what it does not cover (an ADR's "Non-Goals," for example), reread
the whole document against that section after any edit. Flag a claim in the
body that the document's own scope section says is out of bounds.

**Provenance.** Flag any claim, list, or "alternative considered" stated as
fact or history with no verifiable source — no cited code, document, or
measurement behind it. Don't let something pass just because it fits the
expected shape of a section: a "Rejected Alternatives" section doesn't need
a third entry just because similar documents in the project have three, and
an invented-but-plausible alternative that nobody actually proposed
misrepresents how deliberated the decision was.

## Scale: single edit vs. batch rewrite

For one sentence or paragraph, apply the checklist inline and move on.

For a document-level or multi-document rewrite, the draft-then-verify
process matters more, not less — and it works better as its own focused
task (a dedicated session, or a workflow with a separate verification stage)
than folded into a session also doing unrelated content or code work. Style
discipline degrades under split attention: a rule like "no em dash" gets
followed while STE is the active task, then drifts back in during a later,
unrelated edit to the same document, because attention is on the content fix
rather than the style constraint. Isolating the rewrite as its own task
removes that competition.
