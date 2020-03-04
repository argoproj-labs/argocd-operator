# Generating OpenAPI Artifacts

Build the latest openapi-gen from source

``` bash
which openapi-gen > /dev/null || go build -o ~/bin/openapi-gen k8s.io/kube-openapi/cmd/openapi-gen
```

Run the script to generate artifacts

``` bash
hack/generate.sh
```
