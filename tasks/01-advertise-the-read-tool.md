# Advertise the Read Tool

> CodeCrafters · Build Your Own Claude Code · Stage `#aq1`

In this stage, you'll add support for advertising the Read tool in the request.

## Tools

Tools are functions that an LLM can use to perform specific actions, like reading files or running commands.

By default, LLMs cannot access a user's environment, such as their filesystem or terminal. To solve this, Claude Code provides several [tools](https://code.claude.com/docs/en/settings#tools-available-to-claude) (like Read, Write, and Bash) that enable the model to read and modify the user's codebase.

For this stage, you only need to advertise the Read tool (you'll implement it in later stages).

## Advertising tools

Advertising tools lets the model know which tools are available and what arguments they accept.

To advertise tools, edit the existing request in the starter code to add a `tools` array:

```diff
 {
   "model": "...",
   "messages": [...],
+  "tools": [<tool1 spec>, <tool2 spec>, ...]
 }
```

For this stage, the `tools` array should include the Read tool's specification:

```json
{
  "type": "function",
  "function": {
    "name": "Read",
    "description": "Read and return the contents of a file",
    "parameters": {
      "type": "object",
      "properties": {
        "file_path": {
          "type": "string",
          "description": "The path to the file to read"
        }
      },
      "required": ["file_path"]
    }
  }
}
```

The structure consists of the following fields:

- `type` — the type of tool (always `"function"` for tools)
- `function` — contains the function definition
  - `name` — the name of the function (e.g., `"Read"`)
  - `description` — explains the function's purpose and helps the LLM determine when to use it
  - `parameters` — a JSON Schema describing the function's parameters
    - `properties` — defines each parameter (in this case, just `file_path`)
    - `required` — lists which parameters are mandatory

## Tests

The tester will execute your program like this:

```sh
$ ./your_program.sh -p "How many tools are available to you in this request? Number only."
1
```

The tester will verify that:

- Your program outputs a positive number
- Your program exits with exit code 0

## Notes

- You can choose any reasonable name for the Read tool. The tester will only perform end-to-end tests by checking the LLM's response. For example, any of the following names is valid:
  - `Read`
  - `read`
  - `read_file`
  - `ReadFile`, etc.
- For this stage, you only need to advertise the Read tool's availability in the request. We'll handle tool calls in later stages.
- See the [OpenRouter API Specification for Tools](https://openrouter.ai/docs/api/api-reference/chat/create-a-chat-completion#body-tools) for the full API documentation on how to include tools in your request.
