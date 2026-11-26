// Package gen holds the server interface and types generated from
// api/openapi.yaml. Do not edit api.gen.go; change the spec and run make gen.
package gen

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.9.0 -config cfg.yaml ../../../api/openapi.yaml
