#!/bin/sh
set -eu

root_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
compose_file="$root_dir/test/floci/docker-compose.yml"
endpoint="${FLOCI_ENDPOINT:-http://127.0.0.1:4566}"
region="${FLOCI_REGION:-us-east-1}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required" >&2
  exit 2
fi
if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI is required" >&2
  exit 2
fi

export AWS_ENDPOINT_URL="$endpoint"
export AWS_DEFAULT_REGION="$region"
export AWS_REGION="$region"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
unset AWS_PROFILE AWS_DEFAULT_PROFILE AWS_CONFIG_FILE AWS_SHARED_CREDENTIALS_FILE || true

cleanup() {
  docker compose -f "$compose_file" down >/dev/null 2>&1 || true
  if [ -n "${work_dir:-}" ]; then
    rm -rf "$work_dir"
  fi
}
trap cleanup EXIT INT TERM

docker compose -f "$compose_file" up -d

ready=0
seeded=0
i=0
while [ "$i" -lt 60 ]; do
  if aws ec2 describe-regions --all-regions --endpoint-url "$AWS_ENDPOINT_URL" >/dev/null 2>&1; then
    ready=1
    instance_a="$(aws ec2 describe-instances \
      --filters "Name=tag:Name,Values=ssmm-floci-demo-a" \
      --query 'Reservations[0].Instances[0].InstanceId' \
      --output text \
      --endpoint-url "$AWS_ENDPOINT_URL" 2>/dev/null || true)"
    instance_b="$(aws ec2 describe-instances \
      --filters "Name=tag:Name,Values=ssmm-floci-demo-b" \
      --query 'Reservations[0].Instances[0].InstanceId' \
      --output text \
      --endpoint-url "$AWS_ENDPOINT_URL" 2>/dev/null || true)"
    case "$instance_a:$instance_b" in
      i-*:i-*)
        seeded=1
        break
        ;;
    esac
  fi
  i=$((i + 1))
  sleep 1
done
if [ "$ready" -ne 1 ]; then
  echo "Floci did not become ready" >&2
  exit 1
fi
if [ "$seeded" -ne 1 ]; then
  echo "Floci seed hook did not create an EC2 instance" >&2
  exit 1
fi

work_dir="$(mktemp -d)"
mkdir -p "$work_dir/.config/ssmm"
cat >"$work_dir/.config/ssmm/config.toml" <<EOF
[profiles.default]
regions = ["$region"]
EOF

binary="$work_dir/ssmm"
go build -buildvcs=false -o "$binary" "$root_dir/cmd/ssmm"
export HOME="$work_dir"
"$binary" list --region "$region" --output json

fake_bin="$work_dir/bin"
selection_log="$work_dir/selected-instance"
mkdir -p "$fake_bin"
cat >"$fake_bin/aws" <<'EOF'
#!/bin/sh
set -eu

previous=""
target=""
for argument in "$@"; do
  if [ "$previous" = "--target" ]; then
    target="$argument"
  fi
  previous="$argument"
done
printf '%s' "$target" >"$SSMM_SELECTION_LOG"
EOF
cat >"$fake_bin/session-manager-plugin" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod 0755 "$fake_bin/aws" "$fake_bin/session-manager-plugin"
export PATH="$fake_bin:$PATH"
export SSMM_SELECTION_LOG="$selection_log"

"$binary" connect --region "$region"
selected_id="$(cat "$selection_log")"
case "$selected_id" in
  "$instance_a"|"$instance_b") ;;
  *)
    echo "selected instance $selected_id is not one of the seeded instances" >&2
    exit 1
    ;;
esac
echo "selected EC2 instance: $selected_id"
