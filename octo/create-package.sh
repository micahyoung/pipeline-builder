#!/usr/bin/env bash

set -euo pipefail

if [[ "${INCLUDE_DEPENDENCIES}" == "true" ]]; then
  create-package \
    --cache-location "${HOME}"/carton-cache \
    --destination "${HOME}"/buildpack \
    --include-dependencies \
    --version "${VERSION}"
else
  create-package \
    --destination "${HOME}"/buildpack \
    --version "${VERSION}"
fi

[[ -e package.toml ]] && cp package.toml "${HOME}"/package.toml
printf '[buildpack]\nuri = "%s"' "${HOME}"/buildpack >> "${HOME}"/package.toml

if [ -n "${PLATFORM_OS}" ]; then
  printf '[platform]\nos = "%s"' "${PLATFORM_OS}" >> "${HOME}"/package.toml
fi
