package utils

import "testing"

func TestRuleToRegex(t *testing.T) {
	type testDataDefinition struct {
		name           string
		checkedValue   string
		expectedResult string
		expectedError  bool
	}

	tests := []testDataDefinition{
		{
			name:           "simple pattern without wildcards succeeds",
			checkedValue:   "@someone:example.com",
			expectedResult: `^@someone:example\.com$`,
			expectedError:  false,
		},
		{
			name:           "pattern with wildcard as the whole local part succeeds",
			checkedValue:   "@*:example.com",
			expectedResult: `^@([^:@]*):example\.com$`,
			expectedError:  false,
		},
		{
			name:           "pattern with wildcard within the local part succeeds",
			checkedValue:   "@bot.*.something:example.com",
			expectedResult: `^@bot\.([^:@]*)\.something:example\.com$`,
			expectedError:  false,
		},
		{
			name:           "pattern with wildcard as the whole domain part succeeds",
			checkedValue:   "@someone:*",
			expectedResult: `^@someone:([^:@]*)$`,
			expectedError:  false,
		},
		{
			name:           "pattern with wildcard within the domain part succeeds",
			checkedValue:   "@someone:*.organization.com",
			expectedResult: `^@someone:([^:@]*)\.organization\.com$`,
			expectedError:  false,
		},
		{
			name:           "pattern with wildcard in both parts succeeds",
			checkedValue:   "@*:*",
			expectedResult: `^@([^:@]*):([^:@]*)$`,
			expectedError:  false,
		},
		{
			name:           "pattern that does not appear fully-qualified fails",
			checkedValue:   "someone:example.com",
			expectedResult: ``,
			expectedError:  true,
		},
		{
			name:           "pattern that does not appear fully-qualified fails",
			checkedValue:   "@someone",
			expectedResult: ``,
			expectedError:  true,
		},
		{
			name:           "pattern with empty domain part fails",
			checkedValue:   "@someone:",
			expectedResult: ``,
			expectedError:  true,
		},
		{
			name:           "pattern with empty local part fails",
			checkedValue:   "@:example.com",
			expectedResult: ``,
			expectedError:  true,
		},
		{
			name:           "pattern with multiple @ fails",
			checkedValue:   "@someone@someone:example.com",
			expectedResult: ``,
			expectedError:  true,
		},
		{
			name:           "pattern with multiple : fails",
			checkedValue:   "@someone:someone:example.com",
			expectedResult: ``,
			expectedError:  true,
		},
	}

	for _, testData := range tests {
		func(testData testDataDefinition) {
			t.Run(testData.name, func(t *testing.T) {
				actualResult, err := parseMXIDWildcard(testData.checkedValue)

				if testData.expectedError {
					if err != nil {
						return
					}

					t.Errorf("expected an error, but did not get one")
				}

				if err != nil {
					t.Errorf("did not expect an error, but got one: %s", err)
				}

				if actualResult.String() == testData.expectedResult {
					return
				}

				t.Errorf(
					"Expected `%s` to yield `%s`, not `%s`",
					testData.checkedValue,
					testData.expectedResult,
					actualResult.String(),
				)
			})
		}(testData)
	}
}

func TestMatch(t *testing.T) {
	type testDataDefinition struct {
		name           string
		checkedValue   string
		allowedUsers   []string
		expectedResult bool
	}

	tests := []testDataDefinition{
		{
			name:           "Empty allowed users allows anyone",
			checkedValue:   "@someone:example.com",
			allowedUsers:   []string{},
			expectedResult: true,
		},
		{
			name:           "Direct full mxid match is allowed",
			checkedValue:   "@someone:example.com",
			allowedUsers:   []string{"@someone:example.com"},
			expectedResult: true,
		},
		{
			name:           "Direct full mxid match later on is allowed",
			checkedValue:   "@someone:example.com",
			allowedUsers:   []string{"@another:example.com", "@someone:example.com"},
			expectedResult: true,
		},
		{
			name:           "No mxid match is not allowed",
			checkedValue:   "@someone:example.com",
			allowedUsers:   []string{"@another:example.com"},
			expectedResult: false,
		},
		{
			name:           "mxid localpart only wildcard match is allowed",
			checkedValue:   "@someone:example.com",
			allowedUsers:   []string{"@*:example.com"},
			expectedResult: true,
		},
		{
			name:           "mxid localpart with wildcard match is allowed",
			checkedValue:   "@bot.abc:example.com",
			allowedUsers:   []string{"@bot.*:example.com"},
			expectedResult: true,
		},
		{
			name:           "mxid localpart with wildcard match is not allowed when it does not match",
			checkedValue:   "@bot.abc:example.com",
			allowedUsers:   []string{"@employee.*:example.com"},
			expectedResult: false,
		},
		{
			name:           "mxid localpart wildcard for another domain is not allowed",
			checkedValue:   "@someone:example.com",
			allowedUsers:   []string{"@*:another.com"},
			expectedResult: false,
		},
		{
			name:           "mxid domainpart with only wildcard match is allowed",
			checkedValue:   "@someone:example.com",
			allowedUsers:   []string{"@someone:*"},
			expectedResult: true,
		},
		{
			name:           "mxid domainpart with wildcard match is allowed",
			checkedValue:   "@someone:example.organization.com",
			allowedUsers:   []string{"@someone:*.organization.com"},
			expectedResult: true,
		},
		{
			name:           "mxid domainpart with wildcard match is not allowed when it does not match",
			checkedValue:   "@someone:example.another.com",
			allowedUsers:   []string{"@someone:*.organization.com"},
			expectedResult: false,
		},
	}

	for _, testData := range tests {
		func(testData testDataDefinition) {
			t.Run(testData.name, func(t *testing.T) {
				allowedUserRegexes, err := WildcardMXIDsToRegexes(testData.allowedUsers)
				if err != nil {
					t.Error(err)
				}

				actualResult := Match(testData.checkedValue, allowedUserRegexes)

				if actualResult == testData.expectedResult {
					return
				}

				t.Errorf(
					"Expected `%s` compared against `%v` to yield `%v`, not `%v`",
					testData.checkedValue,
					testData.allowedUsers,
					testData.expectedResult,
					actualResult,
				)
			})
		}(testData)
	}
}
