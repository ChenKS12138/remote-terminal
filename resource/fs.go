package resource

import (
	"embed"
)

//go:embed static/*
var ResourceFS embed.FS

