package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func sendMessage(apiKey string, baseUrl string, messages []openai.ChatCompletionMessageParamUnion) *openai.ChatCompletion {

	var readTool = openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "Read",
				Description: openai.String("Read and return the contents of a file"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{
							"type":        "string",
							"description": "The path to the file to read",
						},
					},
					"required": []string{"file_path"},
				},
			},
		},
	}

	var writeTool = openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "Write",
				Description: openai.String("Write content to a file"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{
							"type":        "string",
							"description": "The path to the file to write",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "The content to write to the file",
						},
					},
					"required": []string{"file_path", "content"},
				},
			},
		},
	}

	var bashTool = openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "Bash",
				Description: openai.String("Execute a shell command"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))

	resp, err := client.Chat.Completions.New(context.Background(),
		openai.ChatCompletionNewParams{
			Model:    "anthropic/claude-haiku-4.5",
			Messages: messages,
			Tools:    []openai.ChatCompletionToolUnionParam{readTool, writeTool, bashTool},
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

type Skill struct {
	Command     string // name used to invoke the skill, i.e. the directory name
	Name        string
	Description string
	Body        string
}

func loadSkills() []Skill {
	entries, err := os.ReadDir(filepath.Join(".claude", "skills"))
	if err != nil {
		return nil
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(".claude", "skills", entry.Name(), "SKILL.md"))
		if err != nil {
			continue
		}

		name, description, body := parseSkillFile(string(data))
		if name == "" {
			name = entry.Name()
		}
		skills = append(skills, Skill{
			Command:     entry.Name(),
			Name:        name,
			Description: description,
			Body:        body,
		})
	}

	return skills
}

// parseSkillFile extracts the name/description from the YAML frontmatter and
// the body that follows the closing "---" delimiter.
func parseSkillFile(contents string) (name string, description string, body string) {
	lines := strings.Split(contents, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", strings.TrimSpace(contents)
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}

		key, value, ok := strings.Cut(lines[i], ":")
		if !ok {
			continue
		}

		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			name = value
		case "description":
			description = value
		}
	}

	if end == -1 {
		return name, description, ""
	}

	return name, description, strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
}

func findSkill(skills []Skill, command string) *Skill {
	for i := range skills {
		if strings.EqualFold(skills[i].Command, command) || strings.EqualFold(skills[i].Name, command) {
			return &skills[i]
		}
	}
	return nil
}

// skillsSystemMessage advertises every skill to the model at disclosure level
// 1: name and description only, never the body.
func skillsSystemMessage(skills []Skill) string {
	var b strings.Builder
	b.WriteString("You have access to the following skills:\n\n")
	for _, skill := range skills {
		fmt.Fprintf(&b, "- %s: %s\n", skill.Name, skill.Description)
	}
	return strings.TrimRight(b.String(), "\n")
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

	skills := loadSkills()

	messages := []openai.ChatCompletionMessageParamUnion{}
	if len(skills) > 0 {
		messages = append(messages, openai.SystemMessage(skillsSystemMessage(skills)))
	}

	userContent := prompt
	if strings.HasPrefix(prompt, "/") {
		command, _, _ := strings.Cut(strings.TrimPrefix(prompt, "/"), " ")
		if skill := findSkill(skills, command); skill != nil {
			userContent = skill.Body
		}
	}
	messages = append(messages, openai.UserMessage(userContent))

	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	for {
		resp := sendMessage(apiKey, baseUrl, messages)
		msg := resp.Choices[0].Message

		if len(msg.ToolCalls) == 0 {
			fmt.Print(msg.Content)
			break
		} else {
			// the model is asking us to run a tool.
			assistantParam := msg.ToAssistantMessageParam() // concrete variant - must be a variable
			messages = append(messages, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &assistantParam,
			})

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

					messages = append(messages, openai.ToolMessage(string(contents), toolcall.ID))

				case "Write":
					var args struct {
						FilePath string `json:"file_path"`
						Content  string `json:"content"`
					}

					if err := json.Unmarshal([]byte(toolcall.Function.Arguments), &args); err != nil {
						fmt.Fprintf(os.Stderr, "bad tool arguments: %v\n", err)
						os.Exit(1)
					}

					err := os.WriteFile(args.FilePath, []byte(args.Content), 0644)

					if err != nil {
						fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
						os.Exit(1)
					}

					messages = append(messages, openai.ToolMessage("File written successfully", toolcall.ID))

				case "Bash":
					var args struct {
						Command string `json:"command"`
					}

					if err := json.Unmarshal([]byte(toolcall.Function.Arguments), &args); err != nil {
						fmt.Fprintf(os.Stderr, "bad tool arguments: %v\n", err)
						os.Exit(1)
					}

					out, err := exec.Command("sh", "-c", args.Command).CombinedOutput()

					result := string(out)
					if err != nil {
						result = fmt.Sprintf("command failed: %v\n%s", err, out)
					}
					messages = append(messages, openai.ToolMessage(result, toolcall.ID))
				}
			}

		}
	}

}
