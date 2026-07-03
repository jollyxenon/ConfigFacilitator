## Context

`cfgfc` currently resolves its warehouse root through a single shared helper that always falls back to `~/.configfacilitator` on Unix-like systems or `%USERPROFILE%/.configfacilitator` on native Windows. Every operational command (`new`, `sync`, `switch`, `list`, `apply`, `update`, `reset`, and `revert`) inherits that fixed root, and session convenience state is also stored beneath that root in `.cfgfc-session/`.

The requested change is intentionally smaller than a warehouse migration feature. Users need a persistent built-in way to point `cfgfc` at another warehouse location, while leaving the current default untouched for users who never opt in. The new command must therefore change only where later commands look for warehouse data; it must not copy data between roots or hide which root is active.

## Goals / Non-Goals

**Goals:**

- Add a top-level `cfgfc root` command that inspects or changes the effective warehouse root.
- Keep the existing default root as the fallback when no override has been configured.
- Route every warehouse-scoped command through one effective-root resolver so project discovery, session context, backup state, and link planning all follow the same configured root.
- Keep the implementation bootstrap-friendly by storing the root override outside the warehouse itself.
- Make the user-visible behavior explicit in help text, docs, and tests, including the fact that changing roots does not migrate data.

**Non-Goals:**

- Automatically copy, move, merge, or initialize warehouse contents when the root changes.
- Introduce a nested config subsystem or a general-purpose settings file.
- Add environment-variable precedence or per-command temporary root overrides in this change.
- Add a separate reset-to-default command; users can return to the default by pointing `cfgfc root` back at the default path.

## Decisions

### Add a dedicated `root` command family

The CLI will add `cfgfc root` as a top-level command family with two behaviors:

- `cfgfc root` prints the current effective warehouse root.
- `cfgfc root <path>` normalizes and persists a new effective warehouse root for later invocations.

The command will reject more than one positional path argument.

Rationale: the existing CLI already dispatches single-word command families from the first positional argument. A dedicated `root` command matches that structure, satisfies the request for a persistent standalone command, and keeps parsing straightforward.

Alternative considered: nested forms such as `cfgfc config root <path>`. This would require new parser structure and a larger help-surface change for little benefit.

### Persist the override in a user-scoped bootstrap file outside the warehouse

The effective warehouse root will be resolved from a small bootstrap file stored directly under the current user's home/profile directory, for example `~/.cfgfc-root` on Unix-like systems and `%USERPROFILE%/.cfgfc-root` on native Windows. The file will contain one normalized absolute path. If the file is absent or empty, the resolver falls back to the current default warehouse root.

Rationale: the CLI must discover the warehouse root before it can scan warehouse contents, so the override cannot live inside the warehouse itself. A single-line text file keeps bootstrap parsing obvious and avoids introducing a new configuration schema just to store one path.

Alternative considered: storing the override inside `~/.configfacilitator/` or in a JSON config blob. The former creates a chicken-and-egg dependency, and the latter adds avoidable parsing and migration surface.

### Normalize once at write time, reuse one resolver everywhere

`cfgfc root <path>` will expand supported home/profile shorthands, resolve the input to an absolute cleaned path, and persist that normalized value. The existing shared warehouse-root entry point will then load the bootstrap override first and feed the resulting effective root into every command path that already depends on `scaffold.WarehouseRoot()`.

Rationale: persisting a normalized path makes later command behavior stable across shells and working directories. Reusing one resolver avoids command-specific drift and keeps tests focused on one bootstrap decision point.

Alternative considered: persisting the raw user input. That makes later behavior depend on shell cwd or environment details that may change between invocations.

### Keep root changes non-migratory and non-initializing

Changing the configured root will only update the bootstrap file and report the newly effective path. The command will not copy any project data from the previous root, and it will not create warehouse metadata in the new root beyond whatever later commands already create under current semantics.

Rationale: the user explicitly does not want auto-migration. Keeping `root` side-effect-light also reduces the chance of damaging a populated warehouse by surprise.

Alternative considered: pre-creating the new root or copying the current warehouse automatically. Both would blur the boundary between “change lookup path” and “move my data.”

### Let session context follow the effective root implicitly

`.cfgfc-session/` will remain rooted under the effective warehouse root. After a root change, later commands naturally read session convenience state from the newly selected root rather than from the previous one.

Rationale: session context is warehouse-scoped operational state. Keeping it under the effective root preserves existing structure rules and avoids cross-root convenience-context leakage.

Alternative considered: storing session context separately from the warehouse root. That would make implicit project resolution span unrelated warehouses and would complicate mental models.

## Risks / Trade-offs

- [Risk] Users may point `cfgfc` at an empty or wrong directory and temporarily “lose” their expected projects. → Mitigation: `cfgfc root` prints the effective path plainly, and docs/help will state that changing roots only changes lookup location.
- [Risk] A stale bootstrap file could send every command to the wrong warehouse. → Mitigation: keep the format minimal, keep resolution centralized, and cover override/default resolution with focused tests.
- [Risk] Using a new command family changes help output and command registration expectations. → Mitigation: add command-surface tests and help snapshots alongside workflow tests.
- [Risk] Different roots imply different `.cfgfc-session` stores, which may surprise users who expected `switch` context to carry over. → Mitigation: document that session context is warehouse-local operational state.

## Migration Plan

- No warehouse data migration is required.
- The feature is opt-in: users who never run `cfgfc root <path>` continue using the current defaults.
- Implementation can land incrementally: add bootstrap resolution tests, add CLI parsing/help tests, wire the shared resolver, then add end-to-end alternate-root smoke coverage.
- Rollback is straightforward: remove the `root` command and ignore/delete the bootstrap file. Existing warehouse data remains untouched because the feature never migrates it.

## Open Questions

- None. This change will treat non-existent target roots as valid persisted locations and leave later commands to create or report against that location under their existing semantics.
