---
name: clean-code-go
description: Apply clean code conventions for Go (Robert Martin / Clean Code style, Fowler's code smells, adapted to Go idioms) 
  when writing new Go code or reviewing existing Go code — function size and structure, naming, error handling, interfaces, package structure, and comments. 
  Use whenever the user asks to write, refactor, clean up, or review Go code, or asks "is this good code" / "how should I structure this function" 
  / "where should this package live" in a Go context, even if they don't explicitly mention "clean code."
---

# Clean Code — Go

Concrete, checkable rules distilled from Robert Martin's Clean Code and SOLID principles and Fowler's code smells, adapted for Go idioms 
(Go rejects some of the generic Clean Code advice, notably around error handling — this version reflects that). 
Apply these when writing new Go code and when reviewing/refactoring existing Go code. 
These are defaults, not absolutes — override them if the codebase's existing conventions clearly say otherwise (match the surrounding code's style over these rules), 
and never flag something `gofmt` or `go vet` already enforces (import order, brace placement, spacing) — that's not a code-quality issue, it's a formatting non-issue.

## Functions

- **Do one thing.** A function does one thing if you can't meaningfully extract another function from it. If you find yourself using "and" to describe what a function does, split it.
- **Keep them small.** If a function scrolls past a screen, it's a candidate for extraction. Extract until you can't extract anymore, then stop.
- **One level of abstraction per function.** Don't mix high-level orchestration with low-level detail in the same function.
- **Verb names.** Functions do things: `calculateTotal`, `fetchUser`, `validateInput` — not `total`, `userData`.
- **Max 3 arguments**, fewer is better. More than that → bundle related args into an object/struct.
- **No boolean parameters.** A boolean argument usually means the function does two things depending on its value. Split into two functions instead (e.g. `renderForPrint()` / `renderForScreen()` instead of `render(bool isPrint)`).
- **Output arguments are fine when idiomatic Go.** Prefer return values in general, but don't flag standard Go patterns that rely on output params: `io.Reader.Read(p []byte)`, `UnmarshalJSON(data []byte) error` populating the receiver, or populating a struct passed by pointer. Do flag output params used *instead of* an obvious return value for no idiomatic reason.
- **Command/query separation, with Go's standard exception.** A function either changes state or answers a question — but returning `(value, error)` or `(value, ok)` (map lookups, type assertions, channel receives) is idiomatic Go, not a violation, even though it returns a value alongside a secondary signal.
- **Errors, not exceptions — this is Go's core error-handling model, not a violation of clean code:**
  - Return errors as the last value: `(T, error)`. Never ignore an error by assigning it to `_` unless you can justify why in a comment.
  - Check errors immediately after the call that produced them — don't let them fall through to later code.
  - Wrap errors with context on the way up: `fmt.Errorf("doing x: %w", err)`, not a bare `return err` that loses where it happened.
  - Reserve `panic` for programmer errors and truly unrecoverable states (e.g. failed invariant at startup), never for expected failure paths like "record not found" or "invalid input."
- **Side effects should be obvious from the name**, or avoided. If a function named `checkPassword()` also starts a session as a side effect, that's a trap for the reader.

## Naming

- **Long, descriptive names beat short, cryptic ones.** `elapsedTimeInDays` beats `d`, and beats a comment explaining what `d` means.
- **Name length should scale with scope**, but inversely for different things:
  - Variables: short names are fine only in tiny scopes (e.g. a loop index `i` in a 3-line loop). Wider scope → longer, more descriptive name.
  - Functions/types: short, general names for broadly-used/exported things (`Employee`); longer, specific names as scope narrows.
- **No encodings**: no Hungarian notation (`strName`), no type suffixes (`nameString`), no interface prefixes (`IAccount` — Go interface names should read naturally, see Interfaces below) unless that's an established convention in this codebase.
- **No noise words**: avoid `Manager`, `Processor`, `Data`, `Info` tacked onto type names as filler — they rarely add meaning. Also avoid redundant noise like `Account` vs `AccountInfo` vs `AccountData` existing as distinct-but-confusable types.
- **Disambiguate similar names carefully.** `ActiveAccount()` vs `ActiveAccounts()` — the difference (singular/plural, one letter) is easy to misread. If two names are this close, reconsider one of them.
- **Avoid characters that look alike** in variable names: lowercase `l` vs `1`, `O` vs `0`.
- **Struct/type names are nouns, not plural.** Methods are verbs or verb phrases.
- **Pick one word per concept and stick to it.** Don't mix `get`/`fetch`/`retrieve` for the same kind of operation across a codebase.
- **Don't be cute.** `initializeSystem()` over `bigBang()` — clarity beats personality in a name.
- **Use the problem domain's vocabulary**, not internal jargon, when naming things that map to business concepts.
- **No `Get` prefix on getters.** Go convention: `user.Name()`, not `user.GetName()`.
- **Acronyms stay uniformly cased**: `userID`, `HTTPClient`, `parseURL` — never `userId` or `HttpClient`.
- **No stuttering.** Don't repeat the package name in an exported type: `http.Server`, not `http.HTTPServer`; callers already see `http.` at the call site.
- **Constructors are named, not overloaded** (Go has no overloading): use `NewXxx()` for the standard constructor. For many optional parameters, prefer the functional-options pattern over a long parameter list or boolean flags: `NewServer(addr, WithTimeout(t), WithLogger(l))`.

## Interfaces

- **Small interfaces, strong abstractions.** The Go proverb: "the bigger the interface, the weaker the abstraction." Prefer single-method or few-method interfaces (`io.Reader`, `io.Writer` are the model) over large ones.
- **Name single-method interfaces with an `-er` suffix**: `Reader`, `Writer`, `Closer`, `Validator`.
- **Accept interfaces, return concrete types.** A function's parameters should be interfaces (the minimum needed); its return values should normally be concrete structs, so callers get full access and Go can inline/optimize.
- **Require no more, promise no less.** An implementation must accept everything the interface's contract allows and behave as callers expect — no extra preconditions ("call `Init()` first"), no surprises (nil where callers expect an empty slice, panic where the contract says error). If a caller can tell which implementation it got, the abstraction is leaking.
- **Don't define an interface until there's a real second implementation or a real testing need for one.** A single-implementation interface "for future flexibility" is usually unnecessary indirection — flag this in review.

## Packages

- **One package, one reason to change.** A package is Go's unit of change. If it keeps growing for unrelated reasons, split it — Divergent Change, one level up.
- **No `utils`, `common`, `helpers`, `misc`.** A package named for what it *is* rather than what it *does* has no coherent reason to change, so every unrelated thing lands in it. Name packages for purpose (`encoding/json`, `net/http`) and put code in the package that owns the concept.
- **Keep the import graph wide and flat, not tall.** Push concrete types and wiring up to `main` or the top-level handler; packages underneath take interfaces and stay unaware of which implementation they got. A low-level package importing a high-level one is a dependency pointing the wrong way.
- **Import cycles are a design signal, not just a compile error.** The fix is usually to move the shared concept into its own package, not to merge the two.

## Duplication

- **DRY: flag accidental duplication** — the same logic copy-pasted in two places, which will drift out of sync when one copy is changed and the other is forgotten. This is a fragility risk, not just a style nit.
- Duplication that is *coincidental* (two blocks that happen to look similar today but represent different concepts) is fine to leave alone — don't force an abstraction that couples unrelated things just to remove duplication.

## Comments

- **Prefer to fix the code over explaining it.** A comment explaining confusing code is a signal the code should be clearer, not a justification to leave it as-is.
- **Comments rot.** They aren't checked by a compiler or test, so they silently go out of sync with the code they describe. Don't trust old comments; call out ones that look stale during review.
- **Good comments** (worth keeping/writing):
  - Legal notices (license/copyright headers)
  - Explaining *why*, not *what* — intent that isn't obvious from the code itself
  - Warnings of non-obvious consequences (e.g. "not thread-safe", "O(n²) on large inputs")
  - `TODO` markers for known, intentional gaps
  - Doc comments on exported identifiers — and these must start with the identifier's own name, per Go convention: `// Server handles incoming requests.`, not `// This handles incoming requests.` (`go vet`/`golint` check this — get it right the first time)
- **Bad comments** (flag these in review, remove/replace when writing):
  - Restating what the code obviously does
  - Commented-out code (delete it — source control remembers it)
  - Journal-style change-log comments in the code (that's what commit history is for)
  - Banner/divider comments used to fake structure — extract a function instead
  - Closing-brace comments (`// end if`) — usually means the block is too long; shorten it instead

## Code smells

Fowler's code smells (Refactoring, ch.3), phrased as things to catch yourself doing while drafting — and to spot when reviewing. Each is a heuristic worth a second look, never an automatic violation.

- **Feature Envy** — the function you're writing reads more of another type's fields than its own. → Move it onto the type that owns the data.
- **Data Clumps** — the same two or three parameters keep travelling together through signature after signature. → That's a type asking to be born; declare it and pass one value.
- **Primitive Obsession** — a `string` or `int64` is standing in for a domain concept with its own rules. → Give the concept a named type, so the compiler enforces what a comment otherwise would.
- **Repeated Switches** — you're writing a `switch`/`if`-cascade on a type you already switched on somewhere else. → Replace with an interface method, or one shared map both sites read.
- **Shotgun Surgery** — adding one feature is making you touch five files. → What changes together wants to live together; gather it before adding more.
- **Divergent Change** — the file you're editing keeps growing for unrelated reasons. → Split it so each file has one reason to change.
- **Speculative Generality** — you're adding a parameter, hook, or interface for a case nothing asks for yet. → Leave it out until a real caller needs it (see Interfaces).
- **Message Chains** — you're writing `a.B().C().D()`. → Ask the first object for what you actually need; hide the walk behind one method on it.
- **Middle Man** — the function or type you're adding mostly just forwards to something else. → Call the real target directly; don't add the hop.

## Go gotchas

- **Distinguish absent from zero.** A map lookup returns the zero value for a missing key, so `m[k] == 0` (or `== ""`, `== false`) cannot tell "not configured" from "configured to zero." Wherever the zero value is something a caller could legitimately mean, use comma-ok:

      maxFolders, ok := t.maxFoldersToDelete[station]
      if !ok {
          return defaultMaxFoldersToDelete
      }

  Same trap for optional config parsed into a struct: an unset field and a deliberate `0`/`""` are indistinguishable unless you track presence separately or use a pointer.

## Testing
- We use stretchr/testify for unit tests
- Prefer table-driven tests with one logical assertion per case

Characteristics of a good test:
- Tests behavior users/callers care about
- Prefer exercising behavior through the exported API; test unexported helpers directly only when the logic is genuinely hard to reach from outside
- Survives internal refactors
- Describes WHAT, not HOW

## Logging
- We use tunein/go-logging 
- Make sure to include logging for debugging that will help diagnose problems
- Production captures only Info and Error by default; Debug has to be turned on deliberately for a specific investigation. 
  So anything you'd need to diagnose a failure after the fact belongs at Info — save Debug for detail that's only useful 
  once you're already looking
- Use tags liberally – tag with whatever identifies the unit of work ("group", "broadcast", "station", "bucket", …)
  so logs can be filtered down to the one failing entity. Put those identifiers in tags, not interpolated into the message text
- Use the safelogger package to log Environments, Settings, etc. – anything that may have tokens or keys

## When reviewing code

Sweep the sections above in this order, and cite the specific line or function for every violation — never just "this could be cleaner":

1. Functions — one thing, one abstraction level, arguments, error handling
2. Naming
3. Comments — stale, redundant, commented-out, missing/malformed doc comments
4. Go gotchas — absent vs. zero
5. Interfaces — size, and whether the abstraction earns itself
6. Packages — one reason to change, dependencies pointing inward (only when a package is added or has grown)
7. Duplication
8. Code smells — name the smell when you flag one
9. Testing — coverage of new/changed behavior, and test quality
10. Logging — enough to diagnose in production, identifiers in tags

## When writing new code

Apply these as defaults while drafting — don't write it messy and clean up after. 
In particular: name things properly the first time, keep functions small and single-purpose from the start, 
and don't add a comment where a better name or a small extracted function would do the job instead.

One house preference while drafting: don't combine init and condition — write `v := f()` then `if cond`, 
not `if v := f(); cond`. Applies to new code only; never retrofit it onto existing code or raise it in review.
