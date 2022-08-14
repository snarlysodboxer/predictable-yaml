package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var configFile = flag.String("configFile", "config-example.yaml", "The configuration file")

// config validation errors
var (
	errMissingRegex          = errors.New("Config Validation Error: Regex required")
	errMissingName           = errors.New("Config Validation Error: Name required")
	errOperationsLessThanOne = errors.New("Config Validation Error: at least one Operation required")
	errOpUnknown             = errors.New("Config Validation Error: Operation.Op unknown")
	errRequiredValueUnknown  = errors.New("Config Validation Error: Operation.Value unknown")
	errorAfterValueNotFound  = errors.New("Config Validation Error: after's target not found")
)

// yaml validation errors
var (
	errRequiredExactlyOnce = "Required exactly-once: %w"
	errRequiredTrue        = "Required true: %w"
	errAfter               = "'%s' must be after '%s', is before."
	errMatchesNotUnique    = "Matches must be unique: '%s' and '%s' both matched line '%d'"
)

// ValidationErrors allows us to return multiple validation errors
type ValidationErrors []error

func (errs ValidationErrors) Error() string {
	if len(errs) == 1 {
		return errs[0].Error()
	}

	message := "Multiple validation failures:"
	for _, err := range errs {
		message += "\n" + err.Error()
	}

	return message
}

// Config represents a config for a particular regex
type Config struct {
	Regex      string      `yaml:"regex"`
	Name       string      `yaml:"name"`
	Operations []Operation `yaml:"operations"`
}

// Operation represents an assertion
type Operation struct {
	Op    string `yaml:"op"`
	Value string `yaml:"value"`
}

// Matches represents a map of line number matches for each regex
type Matches map[string][]int

func main() {
	flag.Parse()

	configs, err := getConfigs(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = validateConfigs(configs)
	if err != nil {
		log.Fatal(err)
	}

	filePaths, err := getFilePathsFromStdin()
	if err != nil {
		log.Fatal(err)
	}

	failure := false
	for _, filePath := range filePaths {
		matches, err := getMatches(configs, filePath)
		if err != nil {
			failure = true
			log.Println(err)
		}
		if errs := validate(configs, matches); errs != nil {
			failure = true
			log.Printf("File '%s' has validation errors:\n%v", filePath, getValidationErrorStrings(errs))
		}
	}
	if failure {
		log.Fatal("FAIL")
	}
	log.Println("SUCCESS")
}

func getConfigs(configFile string) (*[]Config, error) {
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	configs := &[]Config{}
	err = yaml.Unmarshal(file, configs)
	if err != nil {
		return nil, err
	}

	return configs, nil
}

func validateConfigs(configs *[]Config) error {
	for _, config := range *configs {
		if config.Regex == "" {
			return errMissingRegex
		}
		if config.Name == "" {
			return errMissingName
		}
		if len(config.Operations) < 1 {
			return errOperationsLessThanOne
		}
		for _, operation := range config.Operations {
			if operation.Op != "required" && operation.Op != "after" {
				return errOpUnknown
			}
			if operation.Op == "required" && operation.Value != "exactly-once" && operation.Value != "true" {
				return errRequiredValueUnknown
			}
			// check for missing after targets
			if operation.Op == "after" {
				_, ok := getConfigForName(configs, operation.Value)
				if !ok {
					return errorAfterValueNotFound
				}
			}
		}
	}

	return nil
}

func getFilePathsFromStdin() ([]string, error) {
	filePaths := make([]string, 0)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) != 0 {
			filePaths = append(filePaths, text)
		} else {
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return filePaths, err
	}
	if len(filePaths) < 1 {
		return filePaths, errors.New("Error: no file paths passed to stdin")
	}

	return filePaths, nil
}

func getMatches(configs *[]Config, filePath string) (Matches, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	matches := make(Matches)
	lineNumber := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, config := range *configs {
			regex, err := regexp.Compile(config.Regex)
			if err != nil {
				return nil, err
			}
			if regex.MatchString(line) {
				matches[config.Name] = append(matches[config.Name], lineNumber)
				lineNumber++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

func validate(configs *[]Config, matches Matches) ValidationErrors {
	errs := matchesAreUnique(matches)
	if len(errs) > 0 {
		return errs
	}

	errs = validateRequireds(configs, matches)
	if len(errs) > 0 {
		return errs
	}

	errs = validateAfters(configs, matches)
	if len(errs) > 0 {
		return errs
	}

	return nil
}

// make sure no regex matched the same line as another regex
func matchesAreUnique(matches Matches) ValidationErrors {
	errs := ValidationErrors{}
	set := make(map[int]string)
	for name, lineNumbers := range matches {
		for _, lineNumber := range lineNumbers {
			if setName, ok := set[lineNumber]; ok {
				err := fmt.Errorf(errMatchesNotUnique, setName, name, lineNumber)
				errs = append(errs, err)
				continue
			}
			set[lineNumber] = name
		}
	}

	return errs
}

// validateRequireds loops through all configs and finds any and all that are 'required', but don't
//   have matches. if any are found, return errors. - significantly simplifies the 'afters' code.
func validateRequireds(configs *[]Config, matches Matches) ValidationErrors {
	errs := ValidationErrors{}
	for _, config := range *configs {
		for _, operation := range config.Operations {
			if operation.Op == "required" {
				lineNumbers := matches[config.Name]
				switch operation.Value {
				case "exactly-once":
					if len(lineNumbers) != 1 {
						err := fmt.Errorf(errRequiredExactlyOnce, fmt.Errorf("'%s', found %d", config.Name, len(lineNumbers)))
						errs = append(errs, err)
					}
				case "true":
					if len(lineNumbers) < 1 {
						err := fmt.Errorf(errRequiredTrue, fmt.Errorf("'%s', found %d", config.Name, len(lineNumbers)))
						errs = append(errs, err)
					}
				}
			}
		}
	}

	return errs
}

// validateAfters loops through all configs and validates all the 'after' ops
func validateAfters(configs *[]Config, matches Matches) ValidationErrors {
	errs := ValidationErrors{}
	for _, config := range *configs {
		for _, operation := range config.Operations {
			if operation.Op == "after" {
				// errs = append(errs, validateAfter(configs, config, operation, matches)...)
				errs = walkAfters(configs, matches, config.Name, operation.Value, errs)
				break
			}
		}
	}

	return errs
}

func walkAfters(configs *[]Config, matches Matches, name, targetName string, errs ValidationErrors) ValidationErrors {
	lineNumbers := matches[name]
	if len(lineNumbers) == 0 {
		// there are no matches for this config (meaning it is also not required)
		return errs
	}
	targetLineNumbers := matches[targetName]
	if len(targetLineNumbers) == 0 {
		// the only way targetLineNumbers is empty is if the target is not required
		// recursively call this function with the target's "after" target
		targetConfig, found := getConfigForName(configs, targetName)
		if !found {
			panic("Should be a config here!") // because we validated configs
		}
		for _, operation := range targetConfig.Operations {
			if operation.Op == "after" {
				return walkAfters(configs, matches, name, operation.Value, errs)
			}
		}
		// the target doesn't have an "after" operation
		return errs
	}
	highestLineNumber := getHighestNumber(lineNumbers)
	highestTargetLineNumber := getHighestNumber(targetLineNumbers)
	if highestLineNumber < highestTargetLineNumber {
		errs = append(errs, fmt.Errorf(errAfter, name, targetName))
	}

	return errs
}

// validateAfter takes a specific config and validates all the 'after' ops
// func validateAfter(configs *[]Config, config Config, operation Operation, matches Matches) []error {
//     errs := []error{}
//     lineNumbers, ok := matches[config.Name]
//     if !ok {
//         // because we first checked all 'required' configs have matches,
//         //    the only way we can get here is if this match is not required
//         //    and has no matches.
//         return nil
//     }
//     targetLineNumbers, ok := matches[operation.Value]
//     if !ok {
//         // because we first checked all 'required' configs have matches,
//         //    the only way we can get here is if the target is not required
//         //    and has no matches.
//         // // TODO check if target has it's own 'after' target and is required, and then check if match is after target's target? recursive
//         // getLowestLineNumberOfTargetsTarget
//         return nil
//     }

//     targetRequired := "false"
//     targetConfig, ok := getConfigForName(configs, operation.Value)
//     if !ok {
//         // we should never get here, because we validated the configs
//         panic("Never get here")
//     }
//     for _, targetOperation := range targetConfig.Operations {
//         if targetOperation.Op == "required" {
//             targetRequired = targetOperation.Value
//         }
//     }
//     switch targetRequired {
//     case "true":
//         // make sure there's at least one match after lowest target lineNumber,
//         //     and no matches before that lineNumber.
//         oneBefore := false
//         atLeastOneAfter := false
//         lowestTargetLineNumber := getLowestNumber(targetLineNumbers)
//         for _, lineNumber := range lineNumbers {
//             if lineNumber < lowestTargetLineNumber {
//                 oneBefore = true
//             }
//             if lineNumber > lowestTargetLineNumber {
//                 atLeastOneAfter = true
//             }
//         }
//         if oneBefore {
//             err := fmt.Errorf(errAfter, fmt.Errorf("'%s' is before '%s'", config.Name, operation.Value))
//             errs = append(errs, err)
//         }
//         if !atLeastOneAfter {
//             // TODO is this error message right?
//             err := fmt.Errorf(errAfter, fmt.Errorf("'%s' is before '%s'", config.Name, operation.Value))
//             errs = append(errs, err)
//         }
//     case "exactly-once":
//         // make sure target lineNumber is higer than all match lineNumbers.
//     case "false":
//         // make sure target lineNumber is higer than any match lineNumbers.
//     }

//     atLeastOneAfter := false
//     for _, lineNumber := range lineNumbers {
//         for _, targetLineNumber := range targetLineNumbers {
//             if lineNumber > targetLineNumber {
//                 atLeastOneAfter = true
//             }
//         }
//     }
//     if !atLeastOneAfter {
//         err := fmt.Errorf(errAfter, fmt.Errorf("'%s' is before '%s'", config.Name, operation.Value))
//         errs = append(errs, err)
//     }

//     return errs
// }

func getConfigForName(configs *[]Config, name string) (Config, bool) {
	for _, config := range *configs {
		if config.Name == name {
			return config, true
		}
	}

	return Config{}, false
}

// func isRequiredForName(configs *[]Config, name string) bool {
//     config, found := getConfigForName(configs, name)
//     if !found {
//         panic("Config not found!")
//     }
//     for _, operation := range config.Operations {
//         if operation.Op == "required" {
//             switch operation.Value {
//             case "true", "exactly-once":
//                 return true
//             }
//             return false
//         }
//     }
//     return false
// }

func getLowestNumber(numbers []int) int {
	if len(numbers) == 0 {
		panic("expected non-empty slice")
	}
	lowest := numbers[0]
	for _, value := range numbers {
		if value < lowest {
			lowest = value
		}
	}

	return lowest
}

func getHighestNumber(numbers []int) int {
	if len(numbers) == 0 {
		panic("expected non-empty slice")
	}
	highest := numbers[0]
	for _, value := range numbers {
		if value > highest {
			highest = value
		}
	}

	return highest
}

func getValidationErrorStrings(errs ValidationErrors) string {
	errorString := ""
	for _, err := range errs {
		errorString += "\t"
		errorString += err.Error()
	}

	return errorString
}
