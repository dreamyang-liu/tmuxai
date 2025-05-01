package internal

import (
	"fmt"
	"time"

	"github.com/alvinunreal/tmuxai/logger"
	"github.com/alvinunreal/tmuxai/system"
	"github.com/briandowns/spinner"
)

// Main function to process regular user messages
// Returns true if the request was accomplished and no further processing should happen
func (m *Manager) ProcessUserMessage(message string) bool {
	// Check if context management is needed before sending
	if m.needSquash() {
		m.Println("Exceeded context size, squashing history...")
		m.squashHistory()
	}

	s := spinner.New(spinner.CharSets[26], 100*time.Millisecond)
	s.Start()

	// check for status change before processing
	if m.Status == "" {
		s.Stop()
		return false
	}

	currentTmuxWindow := m.GetTmuxPanesInXml(m.Config)
	currentMessage := ChatMessage{
		Content:   currentTmuxWindow + "\n\n" + message,
		FromUser:  true,
		Timestamp: time.Now(),
	}

	// build current chat history
	var history []ChatMessage
	switch {
	case m.WatchMode:
		history = []ChatMessage{m.watchPrompt()}
	case m.ExecPane.IsPrepared:
		history = []ChatMessage{m.chatAssistantPreparedPrompt()}
	default:
		history = []ChatMessage{m.chatAssistantPrompt()}
	}

	history = append(history, m.Messages...)

	sending := append(history, currentMessage)

	response, err := m.AiClient.GetResponseFromChatMessages(sending, m.GetOpenRouterModel())
	if err != nil {
		s.Stop()
		m.Status = ""
		fmt.Println("Failed to get response from AI: " + err.Error())
		return false
	}

	// check for status change again
	if m.Status == "" {
		s.Stop()
		return false
	}

	r, err := m.parseAIResponse(response)
	if err != nil {
		s.Stop()
		m.Status = ""
		fmt.Println("Failed to parse AI response: " + err.Error())
		return false
	}

	if m.Config.Debug {
		debugChatMessages(append(history, currentMessage), response)
	}

	logger.Debug("AIResponse: %s", r.String())

	s.Stop()

	responseMsg := ChatMessage{
		Content:   response,
		FromUser:  false,
		Timestamp: time.Now(),
	}

	// did AI follow our guidelines?
	guidelineError, validResponse := m.aiFollowedGuidelines(r)
	if !validResponse {
		m.Println("AI didn't follow guidelines, trying again...")
		m.Messages = append(m.Messages, currentMessage, responseMsg)
		return m.ProcessUserMessage(guidelineError)

	}

	// colorize code blocks in the response
	if r.Message != "" {
		fmt.Println(system.Cosmetics(r.Message))
	}

	// Don't append to history if AI is waiting for the pane or is watch mode no comment
	if r.State == "ExecPaneSeemsBusy" || r.State == "NoComment" {
	} else {
		m.Messages = append(m.Messages, currentMessage, responseMsg)
	}

	// observe mode
	for _, execCommand := range r.ExecCommand {
		code, _ := system.HighlightCode("sh", execCommand)
		m.Println(code)

		isSafe := false
		command := execCommand
		if m.GetExecConfirm() {
			isSafe, command = m.confirmedToExec(execCommand, "Execute this command?", true)
		} else {
			isSafe = true
		}
		if isSafe {
			m.Println("Executing command: " + command)
			system.TmuxSendCommandToPane(m.ExecPane.Id, command, true)
			time.Sleep(1 * time.Second)
		} else {
			m.Status = ""
			return false
		}
	}

	for _, sendKey := range r.SendKeys {
		code, _ := system.HighlightCode("txt", sendKey)
		m.Println(code)

		isSafe := false
		command := sendKey
		if m.GetSendKeysConfirm() {
			isSafe, command = m.confirmedToExec(sendKey, "Send this key(s)?", true)
		} else {
			isSafe = true
		}
		if isSafe {
			m.Println("Sending keys: " + command)
			system.TmuxSendCommandToPane(m.ExecPane.Id, command, false)
			time.Sleep(1 * time.Second)
		} else {
			m.Status = ""
			return false
		}
	}

	if r.State == "ExecPaneSeemsBusy" {
		m.Countdown(m.GetWaitInterval())
		accomplished := m.ProcessUserMessage("waited for 5 more seconds, here is the current pane(s) content")
		if accomplished {
			return true
		}
	}

	// prepared mode
	if r.ExecAndWait != "" {
		code, _ := system.HighlightCode("sh", r.ExecAndWait)
		fmt.Println(code)

		isSafe := false
		command := r.ExecAndWait
		if m.GetExecConfirm() {
			isSafe, command = m.confirmedToExec(r.ExecAndWait, "Execute this command?", true)
		} else {
			isSafe = true
		}
		if isSafe {
			m.ExecWaitCapture(command)
		} else {
			m.Status = ""
			return false
		}
	}

	// observe or prepared mode
	if r.PasteMultilineContent != "" {
		code, _ := system.HighlightCode("txt", r.PasteMultilineContent)
		fmt.Println(code)

		isSafe := false
		if m.GetPasteMultilineConfirm() {
			isSafe, _ = m.confirmedToExec(r.PasteMultilineContent, "Paste multiline content?", false)
		} else {
			isSafe = true
		}

		if isSafe {
			m.Println("Pasting...")
			system.TmuxSendCommandToPane(m.ExecPane.Id, r.PasteMultilineContent, true)
			time.Sleep(1 * time.Second)
		} else {
			m.Status = ""
			return false
		}
	}

	if r.State == "RequestAccomplished" {
		m.Status = ""
		return true
	}

	if r.State == "WaitingForUserResponse" {
		m.Status = "waiting"
		return true // Stop processing, wait for user input
	}

	if r.State == "NoComment" {
		// no comment, do nothing
	}

	if !m.WatchMode {
		accomplished := m.ProcessUserMessage("sending updated pane(s) content")
		if accomplished {
			return true
		}
	}
	return false
}

func (m *Manager) startWatchMode(desc string) {

	// check status
	if m.Status == "" {
		return
	}

	m.Countdown(m.GetWaitInterval())

	accomplished := m.ProcessUserMessage(desc)
	if accomplished {
		m.WatchMode = false
		m.Status = ""
		return
	}

	if m.WatchMode {
		m.startWatchMode(desc)
	}
}

func (m *Manager) aiFollowedGuidelines(r AIResponse) (string, bool) {
	// Ensure only one type of action is present
	numActions := 0
	if len(r.SendKeys) > 0 { numActions++ }
	if len(r.ExecCommand) > 0 { numActions++ }
	if r.PasteMultilineContent != "" { numActions++ }
	if r.ExecAndWait != "" { numActions++ }
	if r.State == "RequestAccomplished" { numActions++ }
	if r.State == "ExecPaneSeemsBusy" { numActions++ }
	if r.State == "WaitingForUserResponse" { numActions++ }
	if r.State == "NoComment" { numActions++ }

	if numActions > 1 {
		return "AI response contains multiple action types. Please provide only one action type.", false
	}

	// If there's a message, no action should be present
	if r.Message != "" && numActions > 0 {
		return "AI response contains both a message and an action. Please provide only one.", false
	}

	// If there's no message, exactly one action should be present
	if r.Message == "" && numActions != 1 {
		return "AI response must contain either a message or exactly one action.", false
	}

	// Validation specific to watch mode
	if m.WatchMode {
		if r.State != "NoComment" && len(r.ExecCommand) == 0 {
			return "In watch mode, AI must either respond with NoComment or ExecCommand.", false
		}
	}

	return "", true
}
