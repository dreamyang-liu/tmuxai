<br/>
<div align="center">
<a href="https://github.com/alvinunreal/tmuxai">
<img src="https://tmuxai.dev/banner.svg" alt="TmuxAI Logo" width="100%" style="border-radius: 10px">
</a>
<p align="center">
AI-Powered, Non-Intrusive Terminal Assistant
<br/>
<br/>
<a href="https://tmuxai.dev/getting-started"><strong>Getting Started »</strong></a>
<br/>
<br/>
<a href="https://tmuxai.dev/screenshots">Screenshots |</a>
<a href="https://github.com/alvinunreal/tmuxai/issues/new?labels=bug&template=bug_report.md">Report Bug |</a>
<a href="https://github.com/alvinunreal/tmuxai/issues/new?labels=enhancement&template=feature_request.md">Request Feature</a>
</p>
</div>

## About The Project

![Product Screenshot](https://tmuxai.dev/shots/vim-docker-compose.png)

TmuxAI is an intelligent terminal assistant that lives inside your tmux sessions. Unlike other CLI AI tools, TmuxAI observes and understands the content of your tmux panes, providing assistance without requiring you to change your workflow or interrupt your terminal sessions.

Think of TmuxAI as a _pair programmer_ that sits beside you, watching your terminal environment exactly as you see it. It can understand what you're working on across multiple panes, help solve problems and execute commands on your behalf in a dedicated execution pane.

### Human-Inspired Interface

TmuxAI's design philosophy mirrors the way humans collaborate at the terminal. Just as a colleague sitting next to you would observe your screen, understand context from what's visible, and help accordingly, TmuxAI:

1. **Observes**: Reads the visible content in all your panes
2. **Communicates**: Uses a dedicated chat pane for interaction
3. **Acts**: Can execute commands in a separate execution pane (with your permission)

This approach provides powerful AI assistance while respecting your existing workflow and maintaining the familiar terminal environment you're already comfortable with.

## Installation

TmuxAI requires only tmux to be installed on your system. It's designed to work on Unix-based operating systems including Linux and macOS.

### Quick Install

The fastest way to install TmuxAI is using the installation script:

```bash
curl -fsSL https://get.tmuxai.dev | bash
```

This installs TmuxAI to `/usr/local/bin/tmuxai` by default. If you need to install to a different location or want to see what the script does before running it, you can view the source at [get.tmuxai.dev](https://get.tmuxai.dev).

### Homebrew

If you use Homebrew, you can install TmuxAI with:

```bash
brew tap alvinunreal/tmuxai
brew install tmuxai
```

### Manual Download

You can also download pre-built binaries from the [GitHub releases page](https://github.com/alvinunreal/tmuxai/releases).

After downloading, make the binary executable and move it to a directory in your PATH:

```bash
chmod +x ./tmuxai
sudo mv ./tmuxai /usr/local/bin/
```

## Post-Installation Setup

After installing TmuxAI, you need to configure your API key to start using it:

1. **Set the API Key**  
   TmuxAI uses the OpenRouter endpoint by default. Set your API key by adding the following to your shell configuration (e.g., `~/.bashrc`, `~/.zshrc`):

   ```bash
   export TMUXAI_OPENROUTER_API_KEY="your-api-key-here"
   ```

2. **Start TmuxAI**

   ```bash
   tmuxai
   ```

3. **Default Configuration**

   By default, TmuxAI is configured to use the OpenRouter endpoint with the `google/gemini-2.5-flash-preview` model. You can customize these settings via the config.yaml file. For details, see the example configuration at [config.example.yaml](https://github.com/alvinunreal/tmuxai/blob/main/config.example.yaml).

## Usage

When you first launch TmuxAI, you will be greeted with a chat interface. You can start using TmuxAI by typing commands in the chat interface.

If you start TmuxAI outside a tmux session, it will create a new tmux session for you.

### Understanding the TmuxAI Layout

TmuxAI is designed to operate within a single tmux window, with one instance of
TmuxAI running per window. It focuses exclusively on the panes within the
current tmux window and does not access or interact with other tmux windows.

TmuxAI organizes your workspace using the following pane structure:

1. **Chat Pane**: This is where you interact with the AI. It features a REPL-like interface with syntax highlighting, auto-completion, and readline shortcuts.

2. **Exec Pane**: TmuxAI selects (or creates) a pane where commands can be executed.

3. **Read-Only Panes**: All other panes in the current window serve as additional context. TmuxAI can read their content but does not interact with them.

### Modes

TmuxAI operates by default in what's called "observe mode". Here's how the interaction flow works:

1. **User types a message** in the Chat Pane.

2. **TmuxAI captures context** from all visible panes in your current tmux window (excluding the Chat Pane itself). This includes:

   - Current command with arguments
   - Detected shell type
   - User's operating system
   - Current content of each pane

3. **TmuxAI processes your request** by sending user's message, the current pane context, and chat history to the AI.

4. **The AI responds** with information, which may include a suggested command to run.

5. **If a command is suggested**, TmuxAI will:

   - Check if the command matches whitelist or blacklist patterns
   - Ask for your confirmation (unless the command is whitelisted)
   - Execute the command in the designated Exec Pane if approved
   - Wait for the `wait_interval` (default: 5 seconds)
   - Capture the new output from all panes
   - Send the updated context back to the AI to continue helping you

6. **The conversation continues** until your task is complete.

### Basic Commands

- `/info`: Display system information.
- `/squash`: Summarize the chat history.
- `/clear`: Clear the chat history.
- `/reset`: Reset the chat history and tmux panes.
- `/exit`: Exit the application.

### Other Modes

- `/watch <description>`: Start watch mode with a description.
- `/prepare`: Start prepare mode

### Configuration

- `/config`: View current configuration.
- `/config set <key> <value>`: Set a configuration value.

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also simply open an issue with the tag "enhancement".
Don't forget to give the project a star! Thanks again!

## License

Distributed under the Apache License. See [Apache License](https://github.com/alvinunreal/tmuxai/blob/main/LICENSE) for more information.
