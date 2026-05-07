/*
Copyright © 2022 david amick git@davidamick.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import "testing"

func TestCountLines(t *testing.T) {
	type testCase struct {
		note      string
		input     string
		separator rune
		expected  int
	}

	testCases := []testCase{
		{
			note:      "empty string",
			input:     "",
			separator: '\n',
			expected:  0,
		},
		{
			note:      "single line no newline",
			input:     "hello",
			separator: '\n',
			expected:  0,
		},
		{
			note:      "single newline",
			input:     "hello\n",
			separator: '\n',
			expected:  1,
		},
		{
			note:      "multiple lines",
			input:     "one\ntwo\nthree\n",
			separator: '\n',
			expected:  3,
		},
		{
			note:      "only newlines",
			input:     "\n\n\n",
			separator: '\n',
			expected:  3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			result := countLines(tc.input, tc.separator)
			if result != tc.expected {
				t.Errorf("countLines(%q, %q) = %d, want %d", tc.input, tc.separator, result, tc.expected)
			}
		})
	}
}
