#!/usr/bin/env bash
# Installs a bunpo release for the GitHub Action: picks the binary for this
# runner, checks it against SHA256SUMS, and puts it on PATH.
#
# Inputs, from action.yml: INPUT_VERSION, ACTION_REF, ACTION_REPOSITORY,
# RUNNER_OS_NAME, RUNNER_ARCH_NAME and GITHUB_TOKEN. BUNPO_RELEASE_DIR takes
# the binaries from a local directory instead of a release, so CI can test
# unreleased builds.
set -euo pipefail

repo=${ACTION_REPOSITORY:-${GITHUB_REPOSITORY:?}}

case $RUNNER_OS_NAME in
Linux) os=linux ;;
macOS) os=darwin ;;
Windows) os=windows ;;
*) echo "::error title=bunpo::unsupported runner OS $RUNNER_OS_NAME" && exit 1 ;;
esac

case $RUNNER_ARCH_NAME in
X64) arch=amd64 ;;
ARM64) arch=arm64 ;;
*) echo "::error title=bunpo::unsupported runner architecture $RUNNER_ARCH_NAME" && exit 1 ;;
esac

ext=""
[ "$os" = windows ] && ext=.exe
asset="bunpo-$os-$arch$ext"

# The version follows the action's ref: an exact tag such as v0.0.3 or
# v0.0.3-rc.1 installs that release, and a major tag such as v0 the newest
# stable v0.x.y, never a v1. Any other ref, such as a branch, installs the
# latest release. The version input overrides all of these.
ref=${ACTION_REF:-}
version=${INPUT_VERSION:-}
tag_pattern='^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'

if [ -n "$version" ] && ! [[ $version =~ $tag_pattern ]]; then
	echo "::error title=bunpo::invalid version $version; expected a release tag such as v0.0.3"
	exit 1
fi

if [ -z "$version" ] && [ -n "${BUNPO_RELEASE_DIR:-}" ]; then
	version=local
elif [ -z "$version" ] && [[ $ref =~ $tag_pattern ]]; then
	version=$ref
elif [ -z "$version" ] && [[ $ref =~ ^v[0-9]+$ ]]; then
	# The API lists releases newest first. Paginate up to 10 pages (1,000
	# releases) so @v0 keeps working even after newer majors ship. The job's
	# token avoids the low limit on anonymous requests from shared runners.
	auth=()
	[ -n "${GITHUB_TOKEN:-}" ] && auth=(-H "Authorization: Bearer $GITHUB_TOKEN")
	for page in 1 2 3 4 5 6 7 8 9 10; do
		# ${auth[@]+...} keeps an empty array working under set -u in bash 3.2,
		# which macOS ships.
		body=$(curl -fsSL --retry 3 ${auth[@]+"${auth[@]}"} "https://api.github.com/repos/$repo/releases?per_page=100&page=$page")
		version=$(printf '%s' "$body" |
			jq -r --arg prefix "$ref." --arg re "$tag_pattern" \
				'map(select(.draft or .prerelease | not) | select((.tag_name | startswith($prefix)) and (.tag_name | test($re)))) | .[0].tag_name // empty')
		[ -n "$version" ] && break
		count=$(printf '%s' "$body" | jq 'length')
		[ "$count" -lt 100 ] && break
	done
	[ -n "$version" ] || {
		echo "::error title=bunpo::no stable $ref.x.y release of $repo"
		exit 1
	}
elif [ -z "$version" ]; then
	latest=$(curl -fsSLI --retry 3 -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
	version=${latest##*/}
	if ! [[ $version =~ $tag_pattern ]]; then
		echo "::error title=bunpo::no published release found for $repo"
		exit 1
	fi
fi

dir="$RUNNER_TEMP/bunpo-$version"
mkdir -p "$dir"

if [ -n "${BUNPO_RELEASE_DIR:-}" ]; then
	cp "$BUNPO_RELEASE_DIR/$asset" "$BUNPO_RELEASE_DIR/SHA256SUMS" "$dir/"
else
	base="https://github.com/$repo/releases/download/$version"
	for file in "$asset" SHA256SUMS; do
		curl -fsSL --retry 3 -o "$dir/$file" "$base/$file" || {
			echo "::error title=bunpo::cannot download $file from release $version of $repo"
			exit 1
		}
	done
fi

# The binary is hashed from standard input: given a Windows path, which holds
# backslashes, sha256sum escapes the name and prefixes the hash with "\".
want=$(awk -v f="$asset" '$2 == f { print $1 }' "$dir/SHA256SUMS")
if command -v sha256sum >/dev/null; then
	got=$(sha256sum <"$dir/$asset" | cut -d' ' -f1)
else
	got=$(shasum -a 256 <"$dir/$asset" | cut -d' ' -f1)
fi

if [ -z "$want" ] || [ "$want" != "$got" ]; then
	echo "::error title=bunpo::$asset from $version does not match SHA256SUMS"
	exit 1
fi

mv "$dir/$asset" "$dir/bunpo$ext"
chmod +x "$dir/bunpo$ext"

echo "$dir" >>"$GITHUB_PATH"
echo "version=$version" >>"$GITHUB_OUTPUT"
echo "bunpo $version installed for $os/$arch"
