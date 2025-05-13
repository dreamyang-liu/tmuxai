package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alvinunreal/tmuxai/config"
	"github.com/alvinunreal/tmuxai/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// AiClient represents an AI client for interacting with various AI providers
type AiClient struct {
	config        *config.OpenRouterConfig
	client        *http.Client
	bedrockClient *bedrockruntime.Client
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents a request to the chat completion API
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// ChatCompletionChoice represents a choice in the chat completion response
type ChatCompletionChoice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

// ChatCompletionResponse represents a response from the chat completion API
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Choices []ChatCompletionChoice `json:"choices"`
}

func NewAiClient(cfg *config.OpenRouterConfig) *AiClient {
	client := &AiClient{
		config: cfg,
		client: &http.Client{},
	}

	// Initialize AWS Bedrock client if provider is bedrock
	if cfg.Provider == "bedrock" {
		// Set default service name if not provided
		serviceName := cfg.ServiceName
		if serviceName == "" {
			serviceName = "bedrock-runtime"
		}

		// Set default region if not provided
		region := cfg.Region
		if region == "" {
			region = "us-west-2"
		}

		// Load AWS configuration
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(region),
		)
		if err != nil {
			logger.Error("Failed to load AWS config: %v", err)
		} else {
			client.bedrockClient = bedrockruntime.NewFromConfig(awsCfg)
			logger.Info("Initialized AWS Bedrock client for region %s", region)
		}
	}

	return client
}

// GetResponseFromChatMessages gets a response from the AI based on chat messages
func (c *AiClient) GetResponseFromChatMessages(ctx context.Context, chatMessages []ChatMessage, model string) (string, error) {
	// Convert chat messages to AI client format
	aiMessages := []Message{}

	for i, msg := range chatMessages {
		var role string

		if i == 0 && !msg.FromUser {
			role = "system"
		} else if msg.FromUser {
			role = "user"
		} else {
			role = "assistant"
		}

		aiMessages = append(aiMessages, Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	logger.Info("Sending %d messages to AI", len(aiMessages))

	// Get response from AI
	response, err := c.ChatCompletion(ctx, aiMessages, model)
	if err != nil {
		return "", err
	}

	return response, nil
}

// ChatCompletion sends a chat completion request to the appropriate AI provider
func (c *AiClient) ChatCompletion(ctx context.Context, messages []Message, model string) (string, error) {
	// Use AWS Bedrock if provider is bedrock
	if c.config.Provider == "bedrock" {
		return c.bedrockChatCompletion(ctx, messages, model)
	}

	// Default to OpenRouter/OpenAI compatible API
	return c.openRouterChatCompletion(ctx, messages, model)
}

// openRouterChatCompletion sends a chat completion request to the OpenRouter/OpenAI compatible API
func (c *AiClient) openRouterChatCompletion(ctx context.Context, messages []Message, model string) (string, error) {
	reqBody := ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		logger.Error("Failed to marshal request: %v", err)
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Remove trailing slash from BaseURL if present: https://github.com/alvinunreal/tmuxai/issues/13
	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	url := baseURL + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqJSON))
	if err != nil {
		logger.Error("Failed to create request: %v", err)
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	req.Header.Set("HTTP-Referer", "https://github.com/alvinunreal/tmuxai")
	req.Header.Set("X-Title", "TmuxAI")

	// Log the request details for debugging before sending
	logger.Debug("Sending API request to: %s with model: %s", url, model)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return "", fmt.Errorf("request canceled: %w", ctx.Err())
		}
		logger.Error("Failed to send request: %v", err)
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response: %v", err)
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Log the raw response for debugging
	logger.Debug("API response status: %d, response size: %d bytes", resp.StatusCode, len(body))

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		logger.Error("API returned error: %s", body)
		return "", fmt.Errorf("API returned error: %s", body)
	}

	// Parse the response
	var completionResp ChatCompletionResponse
	if err := json.Unmarshal(body, &completionResp); err != nil {
		logger.Error("Failed to unmarshal response: %v, body: %s", err, body)
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Return the response content
	if len(completionResp.Choices) > 0 {
		responseContent := completionResp.Choices[0].Message.Content
		logger.Debug("Received AI response (%d characters): %s", len(responseContent), responseContent)
		return responseContent, nil
	}

	// Enhanced error for no completion choices
	logger.Error("No completion choices returned. Raw response: %s", string(body))
	return "", fmt.Errorf("no completion choices returned (model: %s, status: %d)", model, resp.StatusCode)
}

// bedrockChatCompletion sends a chat completion request to AWS Bedrock
func (c *AiClient) bedrockChatCompletion(ctx context.Context, messages []Message, model string) (string, error) {
	if c.bedrockClient == nil {
		return "", fmt.Errorf("AWS Bedrock client not initialized")
	}

	// Determine the model ID to use with Bedrock
	modelID := model
	if modelID == "" {
		modelID = c.config.Model
	}

	// Convert messages to Bedrock Converse API format
	input, err := c.formatBedrockConverseRequest(messages, modelID)
	if err != nil {
		logger.Error("Failed to format Bedrock Converse request: %v", err)
		return "", fmt.Errorf("failed to format Bedrock Converse request: %w", err)
	}

	logger.Debug("Sending Bedrock API request with model: %s", modelID)

	// Send the request to Bedrock
	output, err := c.bedrockClient.Converse(ctx, &input)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return "", fmt.Errorf("request canceled: %w", ctx.Err())
		}
		logger.Error("Failed to invoke Bedrock model: %v", err)
		return "", fmt.Errorf("failed to invoke Bedrock model: %w", err)
	}

	// Parse the response based on the model
	responseContent, err := c.parseBedrockResponse(output)
	if err != nil {
		logger.Error("Failed to parse Bedrock response: %v", err)
		return "", fmt.Errorf("failed to parse Bedrock response: %w", err)
	}

	logger.Debug("Received Bedrock response (%d characters): %s", len(responseContent), responseContent)
	return responseContent, nil
}

// formatBedrockConverseRequest formats messages for the Bedrock Converse API
func (c *AiClient) formatBedrockConverseRequest(messages []Message, modelID string) (bedrockruntime.ConverseInput, error) {
	request := bedrockruntime.ConverseInput{
		ModelId:  aws.String(modelID),
		Messages: []types.Message{},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(8192),
			Temperature: aws.Float32(0.3),
			TopP:        aws.Float32(0.9),
		},
		System: []types.SystemContentBlock{},
	}

	for _, msg := range messages {
		if msg.Role == "system" {
			request.System = append(request.System, &types.SystemContentBlockMemberText{
				Value: msg.Content,
			})
			fmt.Printf("System message added to request: %s\n", msg.Content)
		} else {
			request.Messages = append(request.Messages, types.Message{
				Role: getConversationRole(msg.Role),
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: msg.Content,
					},
				},
			})
		}
	}

	// Use system pompot cache
	request.System = append(request.System, &types.SystemContentBlockMemberCachePoint{
		Value: types.CachePointBlock{
			Type: types.CachePointTypeDefault,
		},
	})

	return request, nil
}

func getConversationRole(role string) types.ConversationRole {
	switch role {
	case "user":
		return types.ConversationRoleUser
	case "assistant":
		return types.ConversationRoleAssistant
	default:
		return types.ConversationRoleAssistant
	}
}

// parseBedrockResponse parses the response from Bedrock based on the model
func (c *AiClient) parseBedrockResponse(output *bedrockruntime.ConverseOutput) (string, error) {
	// type switches can be used to check the union value
	switch v := output.Output.(type) {
	case *types.ConverseOutputMemberMessage:
		return v.Value.Content[0].(*types.ContentBlockMemberText).Value, nil // Value is types.Message

	case *types.UnknownUnionMember:
		return "", fmt.Errorf("unknown tag: %s", v.Tag)

	default:
		return "", fmt.Errorf("union is nil or unknown type")
	}
}

func debugChatMessages(chatMessages []ChatMessage, response string) {

	timestamp := time.Now().Format("20060102-150405")
	configDir, _ := config.GetConfigDir()

	debugDir := fmt.Sprintf("%s/debug", configDir)
	if _, err := os.Stat(debugDir); os.IsNotExist(err) {
		os.Mkdir(debugDir, 0755)
	}

	debugFileName := fmt.Sprintf("%s/debug-%s.txt", debugDir, timestamp)

	file, err := os.Create(debugFileName)
	if err != nil {
		logger.Error("Failed to create debug file: %v", err)
		return
	}
	defer file.Close()

	file.WriteString("==================    SENT CHAT MESSAGES ==================\n\n")

	for i, msg := range chatMessages {
		role := "assistant"
		if msg.FromUser {
			role = "user"
		}
		if i == 0 && !msg.FromUser {
			role = "system"
		}
		timeStr := msg.Timestamp.Format(time.RFC3339)

		file.WriteString(fmt.Sprintf("Message %d: Role=%s, Time=%s\n", i+1, role, timeStr))
		file.WriteString(fmt.Sprintf("Content:\n%s\n\n", msg.Content))
	}

	file.WriteString("==================    RECEIVED RESPONSE ==================\n\n")
	file.WriteString(response)
	file.WriteString("\n\n==================    END DEBUG ==================\n")
}
