package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"encoding/json"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func sendMessage(apiKey string, baseUrl string, messages []openai.ChatCompletionMessageParamUnion) *openai.ChatCompletion {

	var readTool = openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name: "Read",
				Description: openai.String("Read and return the contents of a file"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"file_path":  map[string]any{
							"type": "string",
							"description": "The path to the file to read",
						},
					},
					"required": []string{"file_path"},
				},
			},
		},
	}

	
	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))

	resp, err := client.Chat.Completions.New(context.Background(),
		openai.ChatCompletionNewParams{
			Model: "anthropic/claude-haiku-4.5",
			Messages: messages,
			Tools: []openai.ChatCompletionToolUnionParam{readTool},
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(resp.Choices) == 0 {
		panic("No choices in response")
	}

	return resp
}

func main() {

	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://openrouter.ai/api/v1"
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	messages := []openai.ChatCompletionMessageParamUnion{openai.UserMessage(prompt)}

	
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	for {
		resp := sendMessage(apiKey, baseUrl, messages)
		msg := resp.Choices[0].Message

		if len(msg.ToolCalls) == 0 {
		fmt.Print(msg.Content)
		break;
	} else {
		// the model is asking us to run a tool.
		for _, toolcall := range msg.ToolCalls {
			switch toolcall.Function.Name {
				case "Read": // must match the name we advertised
					var args struct {
						FilePath string `json:"file_path"`
					}
					if err := json.Unmarshal([]byte(toolcall.Function.Arguments), &args); err != nil {
						fmt.Fprintf(os.Stderr, "bad tool arguments: %v\n", err)
						os.Exit(1)
					}

					contents, err := os.ReadFile(args.FilePath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "read failed: %v\n", err)
						os.Exit(1)
					}
					assistantParam := msg.ToAssistantMessageParam() // concrete variant - must be a variable


					messages = append(messages, openai.ChatCompletionMessageParamUnion{
						OfAssistant: &assistantParam, // & requires an addressable value; &f is illegal in Go
					})
					messages = append(messages, openai.ToolMessage(string(contents), toolcall.ID))
			}
		}

		
	}
	}


	

}
