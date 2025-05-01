package internal

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/alvinunreal/tmuxai/logger"
)

func (m *Manager) parseAIResponse(response string) (AIResponse, error) {
	clean := response
	r := AIResponse{}
	cleanForMsg := clean

	// Regular expression to find function calls in the AI response
	// Original pattern used unsupported backreference $1.
	// New pattern captures tag and content, matching general <tag>{content}</closing_tag> structure.
	functionPattern := `<(\w+)>({.*?})</\w+>`
	reFunctions := regexp.MustCompile(functionPattern)
	functionMatches := reFunctions.FindAllStringSubmatch(clean, -1)

	logger.Debug("Found %d function calls in response", len(functionMatches))

	for _, match := range functionMatches {
		if len(match) < 3 {
			continue // Skip invalid match
		}

		functionName := match[1]
		argumentsJSON := match[2]

		// Remove the function call from the message
		cleanForMsg = strings.Replace(cleanForMsg, match[0], "", 1)

		// Parse arguments
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
			continue // Skip if JSON parsing fails
		}

		// Process based on function name
		switch functionName {
		case "TmuxSendKeys":
			if keys, ok := args["keys"].(string); ok {
				r.SendKeys = append(r.SendKeys, keys)
			}
		case "ExecCommand":
			if cmd, ok := args["command"].(string); ok {
				r.ExecCommand = append(r.ExecCommand, cmd)
			}
		case "PasteMultilineContent":
			if content, ok := args["content"].(string); ok {
				r.PasteMultilineContent = content
			}
		case "ExecAndWait":
			if cmd, ok := args["command"].(string); ok {
				r.ExecAndWait = cmd
			}
		case "ChangeState":
			if state, ok := args["state"].(string); ok {
				r.State = state
			}
		}
	}

	// Clean up the message part by removing function calls
	cleanPattern := `<(\w+)>({.*?})</\w+>`
	cleanRegex := regexp.MustCompile(cleanPattern)
	cleanForMsg = cleanRegex.ReplaceAllString(cleanForMsg, "")
	// Also remove any potential leading/trailing whitespace or newlines left after removal
	cleanForMsg = strings.TrimSpace(cleanForMsg)

	r.Message = cleanForMsg

	return r, nil
}
