#!/usr/bin/env bash
set -euo pipefail

# This smoke test exercises the built release binary with an isolated HOME and
# a persisted alternate warehouse root. Repository mutations use cfgfc; direct
# filesystem changes are limited to target-drift and external-sync scenarios.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cfgfc="${CFGFC_BINARY:-$repo_root/dist/cfgfc}"
work="$(mktemp -d)"
home="$work/home"
warehouse="$work/warehouse"
targets="$work/targets"
trap 'rm -rf "$work"' EXIT
mkdir -p "$home" "$warehouse" "$targets"

# run_cfgfc invokes the built binary with the isolated process environment.
run_cfgfc() {
  HOME="$home" "$cfgfc" "$@"
}

# expect_exit verifies one documented failure without hiding its diagnostics.
expect_exit() {
  local expected="$1"
  shift
  local actual
  set +e
  HOME="$home" "$cfgfc" "$@" >"$work/stdout" 2>"$work/stderr"
  actual=$?
  set -e
  if [[ "$actual" -ne "$expected" ]]; then
    cat "$work/stdout" "$work/stderr" >&2
    return 1
  fi
}

# assert_link verifies both ownership destination and readable linked content.
assert_link() {
  local target="$1"
  local source="$2"
  [[ -L "$target" ]]
  [[ "$(readlink "$target")" == "$source" ]]
}

# Select an alternate root and create metadata, targets, file/directory content,
# Mode selections, and active mappings using only cfgfc and standard input.
run_cfgfc root "$warehouse"
run_cfgfc project create OpenCode --description-file - <<'EOF'
Lifecycle smoke project
EOF
run_cfgfc use OpenCode
run_cfgfc column create Models
run_cfgfc column target add Models --dir "$targets/models" --name-from-setting
run_cfgfc setting create Alpha.txt -c Models --kind file --stdin <<'EOF'
alpha-v1
EOF
run_cfgfc column create Skills
run_cfgfc column target add Skills --dir "$targets/skills" --name-from-setting
run_cfgfc setting create Bundle -c Skills --kind directory
run_cfgfc setting content mkdir Bundle prompts -c Skills
run_cfgfc setting content write Bundle prompts/system.md -c Skills --stdin <<'EOF'
system-v1
EOF
run_cfgfc mode create Max
run_cfgfc mode column set Max Models --strategy full
run_cfgfc mode column set Max Skills --strategy full
run_cfgfc apply mode Max

alpha_source="$warehouse/OpenCode/Column/Models/Alpha.txt"
bundle_source="$warehouse/OpenCode/Column/Skills/Bundle"
assert_link "$targets/models/Alpha.txt" "$alpha_source"
assert_link "$targets/skills/Bundle" "$bundle_source"
grep -Fx 'alpha-v1' "$targets/models/Alpha.txt" >/dev/null
grep -Fx 'system-v1' "$targets/skills/Bundle/prompts/system.md" >/dev/null

# Byte-only edits are immediately visible through active links. Adding a new
# full-Mode Setting requires refresh before its new target mapping appears.
run_cfgfc setting content write Alpha.txt -c Models --stdin <<'EOF'
alpha-v2
EOF
grep -Fx 'alpha-v2' "$targets/models/Alpha.txt" >/dev/null
run_cfgfc setting content write Bundle prompts/system.md -c Skills --stdin <<'EOF'
system-v2
EOF
grep -Fx 'system-v2' "$targets/skills/Bundle/prompts/system.md" >/dev/null
run_cfgfc setting create Beta.txt -c Models --kind file --text beta
[[ ! -e "$targets/models/Beta.txt" ]]
run_cfgfc refresh
assert_link "$targets/models/Beta.txt" "$warehouse/OpenCode/Column/Models/Beta.txt"

# Rename every active resource class and verify canonical source paths, context,
# Mode references, and managed links remain valid.
run_cfgfc setting rename Alpha.txt Primary.txt -c Models
run_cfgfc column rename Models Configurations
run_cfgfc mode rename Max Maximum
run_cfgfc project rename OpenCode Code
primary_source="$warehouse/Code/Column/Configurations/Primary.txt"
bundle_source="$warehouse/Code/Column/Skills/Bundle"
assert_link "$targets/models/Primary.txt" "$primary_source"
assert_link "$targets/skills/Bundle" "$bundle_source"
run_cfgfc mode show Maximum -p Code >/dev/null
run_cfgfc status --json | grep -F '"project":"Code"' >/dev/null

# Cascading deletion removes only the selected active Setting and repairs its
# Mode/runtime dependencies. A direct-column apply followed by Mode apply gives
# reset a previous snapshot; one-step revert restores the direct-column state.
run_cfgfc setting delete Beta.txt -p Code -c Configurations --yes --cascade
[[ ! -e "$targets/models/Beta.txt" ]]
[[ -L "$targets/models/Primary.txt" ]]
run_cfgfc apply column Configurations Primary.txt -p Code
[[ ! -e "$targets/skills/Bundle" ]]
run_cfgfc reset -p Code
[[ ! -e "$targets/models/Primary.txt" ]]
run_cfgfc revert -p Code
assert_link "$targets/models/Primary.txt" "$primary_source"
[[ ! -e "$targets/skills/Bundle" ]]

# External disappearance is removed from the index by sync; restoration
# rediscovers the Setting again. These direct source changes are the
# intentional external-interoperability portion of the lifecycle.
run_cfgfc setting create Transient.txt -p Code -c Configurations --kind file --text transient
transient_source="$warehouse/Code/Column/Configurations/Transient.txt"
rm "$transient_source"
run_cfgfc sync -p Code
expect_exit 3 setting show Transient.txt -p Code -c Configurations
cat >"$transient_source" <<'EOF'
restored
EOF
run_cfgfc sync -p Code
run_cfgfc setting show Transient.txt -p Code -c Configurations >/dev/null
rm "$transient_source"
run_cfgfc sync -p Code
expect_exit 3 setting show Transient.txt -p Code -c Configurations

# destructive_case proves target reclamation for one recorded occupied path.
# The unrelated sibling is never part of current mappings and must survive.
destructive_case() {
  local project="$1"
  local occupied_kind="$2"
  local project_targets="$targets/destructive-$project"
  local recorded="$project_targets/Active"
  local unrelated="$project_targets/unrelated"
  local source="$warehouse/$project/Column/Data/Active"

  run_cfgfc project create "$project"
  run_cfgfc column create Data -p "$project"
  run_cfgfc column target add Data -p "$project" --dir "$project_targets" --name-from-setting
  run_cfgfc setting create Active -p "$project" -c Data --kind file --text managed
  run_cfgfc mode create Enabled -p "$project"
  run_cfgfc mode column set Enabled Data -p "$project" --strategy full
  run_cfgfc apply mode Enabled -p "$project"
  assert_link "$recorded" "$source"

  rm "$recorded"
  case "$occupied_kind" in
    file)
      cat >"$recorded" <<'EOF'
unmanaged-file
EOF
      ;;
    symlink)
      cat >"$work/unmanaged-source-$project" <<'EOF'
unmanaged-link-source
EOF
      ln -s "$work/unmanaged-source-$project" "$recorded"
      ;;
    directory)
      mkdir -p "$recorded/nested"
      cat >"$recorded/nested/value" <<'EOF'
unmanaged-directory
EOF
      ;;
  esac
  mkdir -p "$unrelated"
  cat >"$unrelated/keep" <<'EOF'
keep
EOF

  # --cascade and --force-targets do not imply --yes. --yes does not imply
  # --cascade. --yes plus --cascade does not imply --force-targets.
  if [[ "$occupied_kind" == file ]]; then
    expect_exit 5 setting delete Active -p "$project" -c Data --cascade --force-targets
    expect_exit 5 setting delete Active -p "$project" -c Data --yes
  fi
  expect_exit 5 setting delete Active -p "$project" -c Data --yes --cascade
  [[ -e "$recorded" || -L "$recorded" ]]
  [[ -f "$unrelated/keep" ]]

  run_cfgfc setting delete Active -p "$project" -c Data --yes --cascade --force-targets
  [[ ! -e "$recorded" && ! -L "$recorded" ]]
  [[ -f "$unrelated/keep" ]]
}

destructive_case DriftFile file
destructive_case DriftSymlink symlink
destructive_case DriftDirectory directory

run_cfgfc project list >/dev/null
