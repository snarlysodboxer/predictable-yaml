package indentation

import (
	"bufio"
	"bytes"
	"log"
	"regexp"
)

type startStop struct {
	start  int
	stop   int
	spaces int
}

// FixLists can unindent lists in yaml. Expects consistent input indentation.
func FixLists(content []byte, reduceBy int) []byte {
	// create a slice of lines
	lines := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// find lines starting with `- `, and the following line that ends that value
	sequences := []startStop{}
	sequenceStart := regexp.MustCompile(`^\s*- `)
	leadSpaces := regexp.MustCompile(`^\s*`)
	fmtStr := "^"
	for i := 1; i <= reduceBy; i++ {
		fmtStr += " "
	}
	firstNSpaces := regexp.MustCompile(fmtStr)
Lines:
	for index, line := range lines {
		if !sequenceStart.MatchString(line) {
			continue
		}
		ss := startStop{
			start:  index,
			spaces: len(leadSpaces.Find([]byte(line))),
		}
		// check if this is an instance of an existing sequence
		for _, existingSS := range sequences {
			if index > existingSS.start && index <= existingSS.stop && existingSS.spaces == ss.spaces {
				continue Lines
			}
		}

		for innerIndex, innerLine := range lines {
			// start with the first line following sequence start
			if innerIndex <= index {
				continue
			}
			// if the number of spaces is lower, the sequence ended the line before
			innerSpaces := len(leadSpaces.Find([]byte(innerLine)))
			if innerSpaces < ss.spaces {
				ss.stop = innerIndex - 1
				// defense
				if ss.stop < ss.start {
					panic("something went wrong")
				}
				break
			}
		}
		// if an end was not found, then the sequence goes to the end
		if ss.stop == 0 && ss.start > 0 {
			ss.stop = len(lines) - 1
		}
		sequences = append(sequences, ss)
	}

	// convert to map
	newLines := map[int]string{}
	for index, line := range lines {
		newLines[index] = line
	}

	// unindent those lines in the map by reduceBy spaces
	for _, ss := range sequences {
		for i := ss.start; i <= ss.stop; i++ {
			newLines[i] = string(firstNSpaces.ReplaceAll([]byte(newLines[i]), []byte{}))
		}
	}

	// reassemble
	newContent := ""
	for i := 0; i < len(newLines); i++ {
		newContent += newLines[i] + "\n"
	}

	return []byte(newContent)
}
