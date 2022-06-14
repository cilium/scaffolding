#!/bin/bash
# ---
# Ryan Drew, 2022
# Crude script to 'pretty print' a matrix.json into a table.
# ---
# Set default keys if user hasn't provided any
default_keys=(cilium_version kernel num_nodes)
KEYS=(${KEYS:-${default_keys[@]}})

# Quoted, comma separated
_headers=$(printf '"%s", ' "${KEYS[@]}")
headers=${_headers%, }

# jq .key format, comma separated
_getters=$(printf ".%s, " "${KEYS[@]}")
getters=${_getters%, }

jq -r '(['"$headers"'] | (., map(length*"-"))), (.[] | ['"$getters"']) | @tsv' $1 | column -t

