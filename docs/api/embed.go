package apidocs

import "embed"

//go:embed openapi.yaml swagger.html
var files embed.FS

func OpenAPI() []byte {
	data, _ := files.ReadFile("openapi.yaml")
	return data
}

func SwaggerHTML() []byte {
	data, _ := files.ReadFile("swagger.html")
	return data
}
