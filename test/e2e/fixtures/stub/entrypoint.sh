#!/bin/sh
# No-op container used by the e2e suite as a stand-in for the Renovate image.
#
# The operator invokes the Renovate container in two ways:
#   - Discovery init container, with `--write-discovered-repos <path>`: writes
#     a fixed autodiscovery result so the operator's /discovery binary has
#     repositories to turn into GitRepo resources.
#   - Renovate runner container, with no arguments: simply exits successfully.
#
# The discovery result must stay in sync with the repos asserted by
# TestJobExecution in test/e2e/job_test.go.
set -eu

while [ "$#" -gt 0 ]; do
    case "$1" in
        --write-discovered-repos)
            shift
            out="$1"
            mkdir -p "$(dirname "$out")"
            printf '%s\n' '["testorg/repo-a","testorg/repo-b"]' > "$out"
            exit 0
            ;;
        *)
            ;;
    esac
    shift
done

exit 0
