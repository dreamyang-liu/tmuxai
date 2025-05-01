package internal

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/alvinunreal/tmuxai/logger"
)

func (m *Manager) parseAIResponse(response string) (AIResponse, error) {
	r := AIResponse{
		Sequence: make([]ActionStep, 0),
	}
	var lastState string
	currentPos := 0

	// Regex to find the next command tag
	functionPattern := `<(\w+)>({.*?})</\w+>`
	reFunctions := regexp.MustCompile(functionPattern)

	for currentPos < len(response) {
		// Find the next function call starting from currentPos
		loc := reFunctions.FindStringSubmatchIndex(response[currentPos:])
		if loc != nil {
			// Adjust indices relative to the original response string
			loc[0] += currentPos // start of full match
			loc[1] += currentPos // end of full match
			loc[2] += currentPos // start of function name
			loc[3] += currentPos // end of function name
			loc[4] += currentPos // start of arguments JSON
			loc[5] += currentPos // end of arguments JSON
		}

		// 1. Handle text before the tag (or all remaining text if no tag found)
		textEnd := len(response)
		if loc != nil {
			textEnd = loc[0] // Text ends where the tag begins
		}

		if textEnd > currentPos {
			textContent := strings.TrimSpace(response[currentPos:textEnd])
			if textContent != "" {
				r.Sequence = append(r.Sequence, ActionStep{Type: "message", Content: textContent})
				logger.Debug("Parsed message step: %s", textContent)
			}
		}

		// If no more tags found, we're done
		if loc == nil {
			break
		}

		// 2. Process the found tag
		functionName := response[loc[2]:loc[3]]
		argumentsJSON := response[loc[4]:loc[5]]
		logger.Debug("Found tag: %s with args: %s", functionName, argumentsJSON)

		// Parse arguments
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
			logger.Error("Failed to parse JSON arguments for %s: %v. Args: %s", functionName, err, argumentsJSON)
			// Skip this invalid tag by advancing past it
			currentPos = loc[1]
			continue
		}

		// Process based on function name
		switch functionName {
		case "TmuxSendKeys":
			if keys, ok := args["keys"].(string); ok {
				r.Sequence = append(r.Sequence, ActionStep{Type: "sendKeys", Content: keys})
				logger.Debug("Parsed sendKeys step: %s", keys)
			} else {
				logger.Error("Invalid args for TmuxSendKeys: %v", args)
			}
		case "ExecCommand":
			if cmd, ok := args["command"].(string); ok {
				r.Sequence = append(r.Sequence, ActionStep{Type: "execCommand", Content: cmd})
				logger.Debug("Parsed execCommand step: %s", cmd)
			} else {
				logger.Error("Invalid args for ExecCommand: %v", args)
			}
		case "PasteMultilineContent":
			if content, ok := args["content"].(string); ok {
				r.Sequence = append(r.Sequence, ActionStep{Type: "pasteMultiline", Content: content})
				logger.Debug("Parsed pasteMultiline step: %s", content)
			} else {
				logger.Error("Invalid args for PasteMultilineContent: %v", args)
			}
		case "ExecAndWait":
			if cmd, ok := args["command"].(string); ok {
				r.Sequence = append(r.Sequence, ActionStep{Type: "execAndWait", Content: cmd})
				logger.Debug("Parsed execAndWait step: %s", cmd)
			} else {
				logger.Error("Invalid args for ExecAndWait: %v", args)
			}
		case "ChangeState":
			if state, ok := args["state"].(string); ok {
				lastState = state // Update last known state, don't add to sequence
				logger.Debug("Parsed ChangeState: %s", state)
			} else {
				logger.Error("Invalid args for ChangeState: %v", args)
			}
		default:
			logger.Error("Unknown function call found: %s", functionName)
		}

		// 3. Advance current position past the processed tag
		currentPos = loc[1]
	}

	r.State = lastState // Assign the last encountered state
	logger.Debug("Final parsed state: %s", r.State)
	logger.Debug("Final parsed sequence length: %d", len(r.Sequence))

	return r, nil
}
