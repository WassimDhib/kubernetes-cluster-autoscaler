#!/bin/bash

# Description: Build and export the autoscaler Docker image with specified settings.
# Usage: ./image-build.sh <tag_name>

# Exit immediately if a command exits with a non-zero status.
set -e

# Path to the .env file
ENV_FILE=".env"

# Load environment variables from a .env file
load_env() {
  if [ -f "$ENV_FILE" ]; then
    # Export each key=value pair from the .env file
    set -o allexport
    source "$ENV_FILE"
    set +o allexport
    echo "Environment variables loaded from $ENV_FILE"
  else
    echo "Warning: $ENV_FILE not found. Continuing with default environment..."
  fi
}

# Check for required environment variables
check_env_vars() {
  if [[ -z "${gituser}" || -z "${gitpass}" ]]; then
    echo "Error: gituser or gitpass is not set. Ensure they are defined in the .env file."
    exit 1
  fi
  AS_REMOTE_REGISTRY="${as_remote_registry:-as-remote-registry}"
  HTTP_PROXY="${http_proxy:-}"
  HTTPS_PROXY="${https_proxy:-}"
}

# Main logic of the script
main() {
  # Validate the number of script arguments
  if [[ "$#" -ne 1 ]]; then
    echo "Usage: $0 <tag_name>"
    exit 1
  fi

  # Assign the passed argument to tag_name
  tag_name="$1"

  # Load environment variables
  load_env

  # Check the variables
  check_env_vars

  echo "Building Docker image for tag: ${tag_name}"
  sudo docker build \
    --network host \
    --build-arg http_proxy="${HTTP_PROXY}" \
    --build-arg https_proxy="${HTTPS_PROXY}" \
    --build-arg no_proxy=git.example-internal.local \
    --build-arg gituser="${gituser}" \
    --build-arg gitpass="${gitpass}" \
    --build-arg glpat="${glpat}" \
    -f Dockerfile -t registry.example-internal.local/autoscaler:${tag_name} ~/autoscaler_new/kubernetes-cluster-autoscaler/

  echo "Saving Docker image to file..."
  sudo docker save registry.example-internal.local/autoscaler:${tag_name} -o /tmp/autoscaler_${tag_name}.tar.gz
  sudo chmod 755 /tmp/autoscaler_${tag_name}.tar.gz

  echo "Copying Docker image to remote registry..."
  scp /tmp/autoscaler_${tag_name}.tar.gz ${AS_REMOTE_REGISTRY}:/tmp
}

# Run the main function
main "$@"

