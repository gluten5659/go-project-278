package links_test

import (
	"code/internal/db"
	"code/internal/links"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertCreateParameters(
	t *testing.T,
	receivedParameters []db.CreateLinkParams,
	wantShortName string,
) {
	t.Helper()

	generatedNamePattern := fmt.Sprintf(`^[a-zA-Z]{%d}$`, links.ShortNameLength)
	seenNames := make(map[string]bool, len(receivedParameters))

	for _, parameters := range receivedParameters {
		assert.Equal(t, originalURL, parameters.OriginalURL)

		if wantShortName == "" {
			assert.Regexp(t, generatedNamePattern, parameters.ShortName)
		} else {
			assert.Equal(t, wantShortName, parameters.ShortName)
		}

		assert.False(t, seenNames[parameters.ShortName], "reused a short name")

		seenNames[parameters.ShortName] = true
	}
}

func TestCreate(t *testing.T) {
	t.Parallel()

	otherIndexViolation := uniqueViolation("links_pkey")

	testCases := []struct {
		name          string
		shortName     string
		takenNames    int
		queryError    error
		wantCalls     int
		wantShortName string
		wantError     error
	}{
		{
			name:          "creates a link with the requested short name",
			shortName:     shortName,
			wantCalls:     1,
			wantShortName: shortName,
		},
		{
			name:          "reports a taken requested short name without retrying",
			shortName:     shortName,
			takenNames:    1,
			wantCalls:     1,
			wantShortName: shortName,
			wantError:     links.ErrShortNameTaken,
		},
		{
			name:      "generates a short name when none is requested",
			wantCalls: 1,
		},
		{
			name:       "retries when a generated short name is taken",
			takenNames: 1,
			wantCalls:  2,
		},
		{
			name:       "gives up once the attempts run out",
			takenNames: links.ShortNameAttempts,
			wantCalls:  links.ShortNameAttempts,
			wantError:  links.ErrNoFreeShortName,
		},
		{
			name:       "does not retry an unrelated failure",
			queryError: errQueryFailed,
			wantCalls:  1,
			wantError:  errQueryFailed,
		},
		{
			name:       "does not retry a violation of another index",
			queryError: otherIndexViolation,
			wantCalls:  1,
			wantError:  otherIndexViolation,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var receivedParameters []db.CreateLinkParams

			service := links.NewService(stubStore{
				createLink: func(
					_ context.Context,
					parameters db.CreateLinkParams,
				) (db.Link, error) {
					receivedParameters = append(receivedParameters, parameters)

					if testCase.queryError != nil {
						return db.Link{}, testCase.queryError
					}

					if len(receivedParameters) <= testCase.takenNames {
						return db.Link{}, uniqueViolation(links.ShortNameIndex)
					}

					return storedLink(parameters.ShortName), nil
				},
			})

			link, err := service.Create(t.Context(), originalURL, testCase.shortName)

			require.Len(t, receivedParameters, testCase.wantCalls)
			assertCreateParameters(t, receivedParameters, testCase.wantShortName)

			if testCase.wantError != nil {
				require.ErrorIs(t, err, testCase.wantError)

				return
			}

			require.NoError(t, err)

			lastShortName := receivedParameters[len(receivedParameters)-1].ShortName
			assert.Equal(t, storedLink(lastShortName), link)
		})
	}
}
