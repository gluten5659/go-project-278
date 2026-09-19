package api

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

const pathSegmentTag = "path_segment"

var pathSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func newValidator() *validator.Validate {
	validate := validator.New()

	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		jsonName, _, _ := strings.Cut(field.Tag.Get("json"), ",")

		if jsonName == "-" {
			return ""
		}

		return jsonName
	})

	err := validate.RegisterValidation(pathSegmentTag, isPathSegment)
	if err != nil {
		panic(err)
	}

	return validate
}

func isPathSegment(fieldLevel validator.FieldLevel) bool {
	return pathSegmentPattern.MatchString(fieldLevel.Field().String())
}
