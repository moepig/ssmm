#!/bin/sh
set -eu

endpoint="${AWS_ENDPOINT_URL:-http://localhost:4566}"
region="${AWS_DEFAULT_REGION:-us-east-1}"

aws_local() {
  aws --endpoint-url "$endpoint" --region "$region" "$@"
}

seed_instance() {
  name="$1"
  instance_id="$(aws_local ec2 describe-instances \
    --filters "Name=tag:Name,Values=$name" "Name=instance-state-name,Values=pending,running,stopped" \
    --query 'Reservations[0].Instances[0].InstanceId' \
    --output text 2>/dev/null || true)"

  if [ -z "$instance_id" ] || [ "$instance_id" = "None" ]; then
    run_input="{\"ImageId\":\"ami-amazonlinux2023\",\"InstanceType\":\"t2.micro\",\"MinCount\":1,\"MaxCount\":1,\"TagSpecifications\":[{\"ResourceType\":\"instance\",\"Tags\":[{\"Key\":\"Name\",\"Value\":\"$name\"}]}]}"
    instance_id="$(aws_local ec2 run-instances \
      --cli-input-json "$run_input" \
      --query 'Instances[0].InstanceId' \
      --output text)"
  fi

  echo "seeded EC2 instance: $name ($instance_id)"
}

seed_instance ssmm-floci-demo-a
seed_instance ssmm-floci-demo-b
