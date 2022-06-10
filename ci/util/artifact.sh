#!/bin/bash
# ---
# Ryan Drew, 2022
# artifact.sh: Download and list artifacts from CircleCI
#
# Usage: artifact.sh project_slug job_number mode [download_match]
#
# project_slug:   Slug of target project, format is vcs/org_name/project_name
# job_number:     Target job's number (in parens next to job name in webside UI)
# mode:           Either list, which sends json out, or download, which
#                 downloads into currnet directory
# download_match: regex match expression which determines which artifacts are
#                 downloaded. Is matched against the artifact's path.
# ---

set -eo pipefail

PROJECT_SLUG=""
BUILD_NUM=""
API_PREFIX="https://circleci.com/api/v1.1/project/"
DOWNLOAD_MATCH=""

API_URL=""
function make_api_url {
    API_URL="${API_PREFIX}${PROJECT_SLUG}/${BUILD_NUM}/artifacts"
}

LIST=""
function list {
    make_api_url
    LIST=$(curl -s --show-error $API_URL)
}

function download {
    list
    echo $LIST | jq -cj '.[]|., "\n"' | while read line
    do
        path=$(echo $line | jq -r .path)
        if echo $path | grep -E "${DOWNLOAD_MATCH}" -q; then
            url=$(echo $line | jq -r .url)
            echo $line
            echo "----------"
            mkdir -p $(dirname "${path}")
            curl -o $path $url 
        fi
    done
}

PROJECT_SLUG="$1"
BUILD_NUM="$2"
MODE="$3"
DOWNLOAD_MATCH="${4:-.*}"


if [ "${MODE}" = "list" ]; then
    list
    echo $LIST | jq
fi

if [ "${MODE}" = "download" ]; then
    download
fi