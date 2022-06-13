#!/bin/bash
# ---
# Ryan Drew, 2022
# trigger.sh: Trigger main CircleCI pipeline(s)
#
# Makes an API call to trigger the 'setup' pipeline described in
# ".circleci/config.yml", setting the 'run_pipeline' parameter.
#
# Usage: artifact.sh project_slug src_type src_value pipeline
#
# project_slug:   Slug of target project, format is vcs/org_name/project_name
# token:          CircleCI API token.
# src_type:       One of 'branch' or 'tag'. Determines revision of code to 
#                 checkout during the run.
# src_value:      Can be a branch name, tag value, 'pull/<number>/head' where
#                 <number> is a PR number for the PR's ref, or 
#                 'pull/<number>/merge' for the PR's merge ref
# ---

set -eo pipefail

API_PREFIX="https://circleci.com/api/v2/project/"

API_URL=""
function make_api_url {
    API_URL="${API_PREFIX}${PROJECT_SLUG}/pipeline"
}

REQUEST=""
function make_request {
    REQUEST=$(cat << EOF
{
    "${SRC_TYPE}": "${SRC_VALUE}",
    "parameters": {
        "run_pipeline": true,
        "skip_image_build_pipeline": true
    }
}
EOF
)
}

function do_trigger {
    curl --request POST \
        --url $API_URL \
        --header "Circle-Token: $TOKEN" \
        --header 'content-type: application/json' \
        --data "$REQUEST"
}

PROJECT_SLUG="$1"
TOKEN="$2"
SRC_TYPE="$3"
SRC_VALUE="$4"

make_api_url
make_request
do_trigger