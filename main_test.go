package main

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
)

var normalConfigs = &[]Config{
	{Regex: `'^---$`, Name: "document-marker", Operations: []Operation{
		{Op: "required", Value: "exactly-once"},
	}},
	{Regex: `^apiVersion:\s\S`, Name: "apiVersion", Operations: []Operation{
		{Op: "after", Value: "document-marker"},
		{Op: "required", Value: "exactly-once"},
	}},
	{Regex: `^kind:\s\S`, Name: "kind", Operations: []Operation{
		{Op: "after", Value: "apiVersion"},
		{Op: "required", Value: "exactly-once"},
	}},
	{Regex: `^metadata:`, Name: "metadata", Operations: []Operation{
		{Op: "after", Value: "kind"},
		{Op: "required", Value: "true"},
	}},
	{Regex: `^  name:\s\S`, Name: "metadataName", Operations: []Operation{
		{Op: "after", Value: "metadata"},
		{Op: "required", Value: "true"},
	}},
	{Regex: `^  namespace:\s\S`, Name: "metadataNamespace", Operations: []Operation{
		{Op: "after", Value: "metadataName"},
	}},
	{Regex: `^  labels:`, Name: "metadataLabels", Operations: []Operation{
		{Op: "after", Value: "metadataNamespace"},
		{Op: "required", Value: "true"},
	}},
	// TODO add metadataLabelsApp
	{Regex: `^  annotations:`, Name: "metadataAnnotations", Operations: []Operation{
		{Op: "after", Value: "metadataLabels"},
	}},
}

var requiredTrueConfigs = &[]Config{
	{Regex: `'^---$`, Name: "document-marker", Operations: []Operation{
		{Op: "required", Value: "true"},
	}},
}

var unknownRequiredValueConfigs = &[]Config{
	{Regex: `'^---$`, Name: "document-marker", Operations: []Operation{
		{Op: "required", Value: "asdf"},
	}},
}

func TestValidate(t *testing.T) {
	type testCase struct {
		name           string
		expectedErrors ValidationErrors
		Configs        *[]Config
		Matches
	}

	testCases := []testCase{
		{
			name:           "success: normal",
			expectedErrors: nil,
			Configs:        normalConfigs,
			Matches: Matches{
				"document-marker":     []int{0},
				"apiVersion":          []int{1},
				"kind":                []int{2},
				"metadata":            []int{3},
				"metadataName":        []int{4},
				"metadataNamespace":   []int{5},
				"metadataLabels":      []int{6},
				"metadataAnnotations": []int{7},
			},
		},
		{
			name:           "success: normal without namespace",
			expectedErrors: nil,
			Configs:        normalConfigs,
			Matches: Matches{
				"document-marker":     []int{0},
				"apiVersion":          []int{1},
				"kind":                []int{2},
				"metadata":            []int{3},
				"metadataName":        []int{4},
				"metadataLabels":      []int{6},
				"metadataAnnotations": []int{7},
			},
		},
		{
			name:           "fail: missing, when required exactly-once",
			expectedErrors: ValidationErrors{fmt.Errorf(errRequiredExactlyOnce, fmt.Errorf("'document-marker', found 0"))},
			Configs:        normalConfigs,
			Matches: Matches{
				"apiVersion":          []int{1},
				"kind":                []int{2},
				"metadata":            []int{3},
				"metadataName":        []int{4},
				"metadataNamespace":   []int{5},
				"metadataLabels":      []int{6},
				"metadataAnnotations": []int{7},
			},
		},
		{
			name:           "fail: more than one, when required exactly-once",
			expectedErrors: ValidationErrors{fmt.Errorf(errRequiredExactlyOnce, fmt.Errorf("'apiVersion', found 2"))},
			Configs:        normalConfigs,
			Matches: Matches{
				"document-marker":     []int{0},
				"apiVersion":          []int{1, 5},
				"kind":                []int{2},
				"metadata":            []int{3},
				"metadataName":        []int{4},
				"metadataNamespace":   []int{6},
				"metadataLabels":      []int{7},
				"metadataAnnotations": []int{8},
			},
		},
		{
			name:           "fail: does not exist",
			expectedErrors: ValidationErrors{fmt.Errorf(errRequiredTrue, fmt.Errorf("'document-marker', found 0"))},
			Configs:        requiredTrueConfigs,
			Matches:        Matches{},
		},
		{
			name:           "fail:  match should be after target, is before",
			expectedErrors: ValidationErrors{fmt.Errorf(errAfter, "apiVersion", "document-marker")},
			Configs:        normalConfigs,
			Matches: Matches{
				"document-marker":     []int{1},
				"apiVersion":          []int{0},
				"kind":                []int{2},
				"metadata":            []int{3},
				"metadataName":        []int{4},
				"metadataNamespace":   []int{5},
				"metadataLabels":      []int{6},
				"metadataAnnotations": []int{7},
			},
		},
		{
			name:           "success: match should be after target, target is before and after match",
			expectedErrors: nil,
			Configs:        normalConfigs,
			Matches: Matches{
				"document-marker":     []int{0},
				"apiVersion":          []int{1},
				"kind":                []int{2},
				"metadata":            []int{5, 3},
				"metadataName":        []int{4},
				"metadataNamespace":   []int{6},
				"metadataLabels":      []int{7},
				"metadataAnnotations": []int{8},
			},
		},
		{
			name:           "fail: match should be after target, match is before and after target",
			expectedErrors: ValidationErrors{fmt.Errorf(errAfter, "metadataName", "metadata")},
			Configs:        normalConfigs,
			Matches: Matches{
				"document-marker":     []int{0},
				"apiVersion":          []int{1},
				"kind":                []int{2},
				"metadata":            []int{4},
				"metadataName":        []int{5, 3},
				"metadataNamespace":   []int{6},
				"metadataLabels":      []int{7},
				"metadataAnnotations": []int{8},
			},
		},
		{
			name:           "success: is before and after, when should be after 2",
			expectedErrors: nil,
			Configs:        normalConfigs,
			Matches: Matches{
				"document-marker":     []int{0},
				"apiVersion":          []int{1},
				"kind":                []int{2},
				"metadata":            []int{6, 3},
				"metadataName":        []int{4, 5},
				"metadataNamespace":   []int{7},
				"metadataLabels":      []int{8},
				"metadataAnnotations": []int{9},
			},
		},
		// TODO match after non-existent walks up requireds tree
		// TODO many validation errors at once
	}

	for _, tc := range testCases {
		// ensure we maintain valid test configs
		err := validateConfigs(tc.Configs)
		if err != nil {
			t.Fatalf("For test '%s': Failed to validate test configs: %v", tc.name, err)
		}

		// do it
		errs := validate(tc.Configs, tc.Matches)
		if len(errs) != len(tc.expectedErrors) || !validationErrorsMatch(errs, tc.expectedErrors) {
			expected := getValidationErrorStrings(tc.expectedErrors)
			got := getValidationErrorStrings(errs)
			t.Errorf("Description: %s: main.validate(...): -expected, +got:\n-%#v\n+%#v\n", tc.name, expected, got)
		}

	}
}

func getValidConfigs() *[]Config {
	return &[]Config{
		{
			Regex: "^---$",
			Name:  "document-marker",
			Operations: []Operation{
				{
					Op:    "required",
					Value: "exactly-once",
				},
			},
		},
		{
			Regex: `^apiVersion:\s\S`,
			Name:  "apiVersion",
			Operations: []Operation{
				{
					Op:    "after",
					Value: "document-marker",
				},
				{
					Op:    "required",
					Value: "exactly-once",
				},
			},
		},
	}
}

func TestValidateConfigs(t *testing.T) {
	// valid
	configs := getValidConfigs()
	err := validateConfigs(configs)
	if err != nil {
		t.Error(err)
	}

	// missing regex
	configs = getValidConfigs()
	(*configs)[0].Regex = ""
	err = validateConfigs(configs)
	if !errors.Is(err, errMissingRegex) {
		t.Error("Expected error, got none")
	}

	// missing name
	configs = getValidConfigs()
	(*configs)[0].Name = ""
	err = validateConfigs(configs)
	if !errors.Is(err, errMissingName) {
		t.Error("Expected error, got none")
	}

	// operations less than 1
	configs = getValidConfigs()
	(*configs)[0].Operations = []Operation{}
	err = validateConfigs(configs)
	if !errors.Is(err, errOperationsLessThanOne) {
		t.Error("Expected error, got none")
	}

	// op other than 'required' or 'after'
	configs = getValidConfigs()
	(*configs)[0].Operations = []Operation{
		{
			Op:    "no-exist",
			Value: "exactly-once",
		},
	}
	err = validateConfigs(configs)
	if !errors.Is(err, errOpUnknown) {
		t.Error("Expected error, got none")
	}

	// value other than 'exactly-once' or 'true'
	configs = getValidConfigs()
	(*configs)[0].Operations = []Operation{
		{
			Op:    "required",
			Value: "asdf",
		},
	}
	err = validateConfigs(configs)
	if !errors.Is(err, errRequiredValueUnknown) {
		t.Error("Expected error, got none")
	}

	// op 'after', w/ value pointing to non-existent target
	configs = getValidConfigs()
	(*configs)[1].Operations = []Operation{
		{
			Op:    "after",
			Value: "asdf",
		},
	}
	err = validateConfigs(configs)
	if !errors.Is(err, errorAfterValueNotFound) {
		t.Errorf("Expected '%v' error, got '%v'", errRequiredValueUnknown, err)
	}
}

func TestGetLowestNumber(t *testing.T) {
	number := getLowestNumber([]int{1})
	if number != 1 {
		t.Errorf("Expected '1', got '%d'", number)
	}
	number = getLowestNumber([]int{1, 2, 3})
	if number != 1 {
		t.Errorf("Expected '1', got '%d'", number)
	}
	number = getLowestNumber([]int{3, 2, 1})
	if number != 1 {
		t.Errorf("Expected '1', got '%d'", number)
	}
	number = getLowestNumber([]int{3, 2, 0})
	if number != 0 {
		t.Errorf("Expected '0', got '%d'", number)
	}
	number = getLowestNumber([]int{3, 2, 1000})
	if number != 2 {
		t.Errorf("Expected '2', got '%d'", number)
	}
}

func TestMatchesAreUnique(t *testing.T) {
	matches := Matches{}
	errs := matchesAreUnique(matches)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got '%#v'", errs)
	}

	matches = Matches{
		"asdf": []int{1, 2},
		"fdsa": []int{3, 4},
	}
	errs = matchesAreUnique(matches)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got '%#v'", errs)
	}

	matches = Matches{
		"asdf": []int{1, 2},
		"fdsa": []int{3, 4, 2},
	}
	errs = matchesAreUnique(matches)
	err := fmt.Errorf(errMatchesNotUnique, "asdf", "fdsa", 2)
	if len(errs) != 1 || errs[0].Error() != err.Error() {
		t.Errorf("Expected '%v' error, got '%v'", err, errs)
	}
}

func validationErrorsMatch(one, other ValidationErrors) bool {
	if len(one) != len(other) {
		return false
	}
	for index := range one {
		if one[index].Error() != other[index].Error() {
			return false
		}
	}

	return true
}
