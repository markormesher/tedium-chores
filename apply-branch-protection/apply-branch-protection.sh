#!/usr/bin/env bash
set -euo pipefail

cd /tedium/repo

check_name=""
if grep "ci-all" .drone.yml >/dev/null 2>&1; then
  check_name="continuous-integration/drone/push"
elif grep "ci-all" .circleci/config.yml >/dev/null 2>&1; then
  check_name="ci/circleci: ci-all"
fi

if [[ "${check_name}" == "" ]]; then
  echo "No ci-all stage found"
  exit 0
fi

if [[ "${TEDIUM_PLATFORM_TYPE}" == "gitea" ]]; then
  cat <<EOF > payload.json
{
  "rule_name": "${TEDIUM_REPO_DEFAULT_BRANCH}",
  "branch_name": "${TEDIUM_REPO_DEFAULT_BRANCH}",
  "enable_push": false,
  "enable_status_check": true,
  "status_check_contexts": [
    "${check_name}
  ],
  "required_approvals": 0,
  "dismiss_stale_approvals": true
}
EOF

  curl -X POST \
    --verbose \
    --fail \
    --header "content-type: application/json" \
    --data @payload.json \
    -H "Authorization: Bearer ${TEDIUM_PLATFORM_TOKEN}" \
    "${TEDIUM_PLATFORM_API_BASE_URL}/repos/${TEDIUM_REPO_OWNER}/${TEDIUM_REPO_NAME}/branch_protections"
fi

if [[ "${TEDIUM_PLATFORM_TYPE}" == "github" ]]; then
  cat <<EOF > payload.json
{
  "enforce_admins": false,
  "allow_force_pushes": false,
  "restrictions": null,
  "required_pull_request_reviews": {
    "required_approving_review_count": 0,
    "dismiss_stale_reviews": true
  },
  "required_status_checks": {
    "strict": false,
    "contexts": [
      "${check_name}"
    ]
  }
}
EOF

  curl -X PUT \
    --verbose \
    --fail \
    --header "content-type: application/json" \
    --data @payload.json \
    -H "Authorization: Bearer ${TEDIUM_PLATFORM_TOKEN}" \
    "${TEDIUM_PLATFORM_API_BASE_URL}/repos/${TEDIUM_REPO_OWNER}/${TEDIUM_REPO_NAME}/branches/${TEDIUM_REPO_DEFAULT_BRANCH}/protection"
fi
