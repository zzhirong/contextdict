#!/bin/bash

if [ -z "$DS_API_KEY" ]; then
    echo "Error: DS_API_KEY environment variable is required"
    exit 1
fi

helm upgrade --install contextdict ./contextdict \
    --set-string secrets.dsApiKey="$DS_API_KEY" \
    --set image.tag="v0.0.8"
