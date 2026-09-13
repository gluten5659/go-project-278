package api_test

import (
	"code/internal/api"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	createRequestStructName = "createLinkRequest"
	updateRequestStructName = "updateLinkRequest"

	originalURLField = "original_url"

	requiredTag  = "required"
	httpURLTag   = "http_url"
	minLengthTag = "min"
	maxLengthTag = "max"

	malformedBody         = `{"original_url":`
	invalidURLBody        = `{"original_url": "example.com", "short_name": "example"}`
	shortNameTooShortBody = `{"original_url": "https://example.com", "short_name": "ab"}`
	shortNameTooLongBody  = `{"original_url": "https://example.com",` +
		` "short_name": "abcdefghijklmnopqrstuvwxyz0123456789"}`
	everyFieldInvalidBody = `{"original_url": "example.com", "short_name": "ab"}`
)

func fieldValidationMessage(structName string, field string, failedTag string) string {
	return fmt.Sprintf(
		"Key: '%s.%s' Error:Field validation for '%s' failed on the '%s' tag",
		structName,
		field,
		field,
		failedTag,
	)
}

func invalidFieldsBody(
	t *testing.T,
	structName string,
	failedTagsByField map[string]string,
) string {
	t.Helper()

	messagesByField := make(map[string]string, len(failedTagsByField))

	for field, failedTag := range failedTagsByField {
		messagesByField[field] = fieldValidationMessage(structName, field, failedTag)
	}

	body, err := json.Marshal(map[string]map[string]string{"errors": messagesByField})
	require.NoError(t, err)

	return string(body)
}

func TestLinkPayloadValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "rejects a malformed body on create",
			method:     http.MethodPost,
			path:       collectionPath,
			body:       malformedBody,
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects an empty body on create",
			method:     http.MethodPost,
			path:       collectionPath,
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects a create without an original url",
			method:     http.MethodPost,
			path:       collectionPath,
			body:       `{"short_name": "example"}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				originalURLField: requiredTag,
			}),
		},
		{
			name:       "rejects a create with an original url that is not http",
			method:     http.MethodPost,
			path:       collectionPath,
			body:       invalidURLBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				originalURLField: httpURLTag,
			}),
		},
		{
			name:       "rejects a create with a short name below the minimum length",
			method:     http.MethodPost,
			path:       collectionPath,
			body:       shortNameTooShortBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				api.ShortNameField: minLengthTag,
			}),
		},
		{
			name:       "rejects a create with a short name above the maximum length",
			method:     http.MethodPost,
			path:       collectionPath,
			body:       shortNameTooLongBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				api.ShortNameField: maxLengthTag,
			}),
		},
		{
			name:       "reports every invalid field of a create at once",
			method:     http.MethodPost,
			path:       collectionPath,
			body:       everyFieldInvalidBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, createRequestStructName, map[string]string{
				originalURLField:   httpURLTag,
				api.ShortNameField: minLengthTag,
			}),
		},
		{
			name:       "rejects a malformed body on update",
			method:     http.MethodPut,
			path:       linkPath,
			body:       malformedBody,
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects an empty body on update",
			method:     http.MethodPut,
			path:       linkPath,
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantBody:   invalidRequestBody,
		},
		{
			name:       "rejects an update without an original url",
			method:     http.MethodPut,
			path:       linkPath,
			body:       `{"short_name": "example"}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				originalURLField: requiredTag,
			}),
		},
		{
			name:       "rejects an update with an original url that is not http",
			method:     http.MethodPut,
			path:       linkPath,
			body:       invalidURLBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				originalURLField: httpURLTag,
			}),
		},
		{
			name:       "rejects an update without a short name",
			method:     http.MethodPut,
			path:       linkPath,
			body:       namelessBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				api.ShortNameField: requiredTag,
			}),
		},
		{
			name:       "rejects an update with a short name below the minimum length",
			method:     http.MethodPut,
			path:       linkPath,
			body:       shortNameTooShortBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				api.ShortNameField: minLengthTag,
			}),
		},
		{
			name:       "rejects an update with a short name above the maximum length",
			method:     http.MethodPut,
			path:       linkPath,
			body:       shortNameTooLongBody,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: invalidFieldsBody(t, updateRequestStructName, map[string]string{
				api.ShortNameField: maxLengthTag,
			}),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			recorder := performRequest(
				t,
				stubQuerier{},
				testCase.method,
				testCase.path,
				testCase.body,
			)

			assertResponse(t, recorder, testCase.wantStatus, testCase.wantBody)
		})
	}
}
