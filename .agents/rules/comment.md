---
description: Comment conventions for source files
globs:
  - "**/*.go"
  - "**/*.sh"
  - "**/*.yml"
  - "**/*.yaml"
paths:
  - "**/*.go"
  - "**/*.sh"
  - "**/*.yml"
  - "**/*.yaml"
---

# Comment conventions

RULE: Inline comments explain "why", not "what". Doc comments state what a
caller can rely on, not how the body does it. Do not narrate steps visible in
the body, repeat the signature, or restate the next line.

RULE: Keep comments concise. Write down what the code can't tell the reader,
and end the comment there. One sentence is often enough. Leave out call order,
caller workflow, and analogies to sibling code unless they are part of the
contract. Describe the code as it is now, not how it changed across edits.
Words like "now", "no longer", "instead of", or "previously" only make sense
to someone who saw the earlier version. Agent-written comments trend long,
with "because…" and "which…" clauses that justify every fact, so reread and
tighten them before finishing the change.

RULE: Add a short comment when code looks odd or unnecessary but is needed. Say
what requires it, such as a Temporal Cloud API rule, and what would break
without it. Skip the comment when the code already makes the reason clear.

- BAD: "Requeue if the namespace is not active."
- GOOD: "Temporal Cloud rejects a delete until the Namespace is Active, so
  check again later."

RULE: Avoid the following phrases and instead use the simpler, more
straightforward alternatives:

- Avoid "<subject> carries the <target>".
  - BAD: "Annotation carries the resource identifier"
  - GOOD: "`cloud.temporal.io/resource-id` contains the resource identifier".
- Avoid "the verdict stays with the prior decision".
  - BAD: "If the resource was adopted, the verdict stays with the prior
    decision".
  - GOOD: "If the resource was adopted, simply return nil".

RULE: Write prose comments as plain sentences with a subject. Do not use em
dashes, colons, semicolons, or `**Header**: text` labels. Use backticks only to
mark an identifier. Keep punctuation that directives, URLs, or code syntax
require.

RULE: Make sure a comment is true before you write it. Read the code and tests
first. Check that you named the right thing as doing the work and that you
didn't overstate what the code saves or prevents. Don't describe what other files
or functions do, because they can change without anyone updating this comment.

RULE: Preserve the author's wording when editing an existing comment. Correct
inaccurate claims, and add a missing contract when the code changes.
