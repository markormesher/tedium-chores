package lanuages

func cacheLoadCommand() string {
	return `
if [[ "${CI_CACHE_TOKEN:-x}" != x ]] && [[ -f ".task-meta-cache-key" ]]; then
  request_key=$(cat ".task-meta-cache-key" | tr -d '\r\n')
  start_ts=$(date +%s)
  header_file=$(mktemp)
  cache_file=$(mktemp)

  echo "loading cache..."
  echo "request key: ${request_key}"
  if curl -fsSL -D "${header_file}" -H "authorization: Bearer ${CI_CACHE_TOKEN}" "https://ci-cache.markormesher.co.uk/cache/${request_key}" -o "${cache_file}"; then
    actual_key=$(cat "${header_file}" | grep -i x-cache-key | cut -d ' ' -f 2 | tr -d '\r\n')
    size=$(du -h "${cache_file}" | awk '{ print $1 }')
    echo "received key: ${actual_key}"
    echo "received size: ${size}"

    if [[ "${actual_key}" == "${request_key}" ]]; then
      touch .task-meta-cache-exact-match
    fi

    echo "unpacking cache..."
    tar xzP -f "${cache_file}" || true
  else
    echo "no cache loaded"
  fi

  rm -f "${cache_file}" "${header_file}"
  end_ts=$(date +%s)
  echo "loading cache took $(( $end_ts - $start_ts ))s"
fi
`
}

func cacheSaveCommand() string {
	return `
if [[ "${CI_CACHE_TOKEN:-x}" != x ]] && [[ -f ".task-meta-cache-key" ]]; then
  if [[ -f .task-meta-cache-exact-match ]]; then
    echo "skipping re-upload because an exact match was returned from the cache"
    exit 0
  fi

  request_key=$(cat ".task-meta-cache-key")
  start_ts=$(date +%s)
  cache_file=$(mktemp)

  echo "packing cache..."
  tar czP -f "${cache_file}" $(cat .task-meta-cache-paths)
  size=$(du -h "${cache_file}" | awk '{ print $1 }')

  echo "uploading cache (${size})..."
  if curl -fsSL -H "authorization: Bearer ${CI_CACHE_TOKEN}" -X PUT "https://ci-cache.markormesher.co.uk/cache/${request_key}" --data-binary "@${cache_file}"; then
    echo "uploaded cache"
  else
    echo "error saving cache"
  fi

  rm -f "${cache_file}"
  end_ts=$(date +%s)
  echo "saving cache took $(( $end_ts - $start_ts ))s"
fi
`
}
