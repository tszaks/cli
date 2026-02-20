package cursor

import "encoding/json"

// CursorHooksFile represents the .cursor/HooksFileName structure.
// Cursor uses a flat JSON file with version and hooks sections.
//
//nolint:revive // CursorHooksFile is clearer than HooksFile when used outside this package
type CursorHooksFile struct {
	Version int         `json:"version"`
	Hooks   CursorHooks `json:"hooks"`
}

// CursorHooks contains all hook configurations using camelCase keys.
//
//nolint:revive // CursorHooks is clearer than Hooks when used outside this package
type CursorHooks struct {
	SessionStart       []CursorHookEntry `json:"sessionStart,omitempty"`
	SessionEnd         []CursorHookEntry `json:"sessionEnd,omitempty"`
	BeforeSubmitPrompt []CursorHookEntry `json:"beforeSubmitPrompt,omitempty"`
	Stop               []CursorHookEntry `json:"stop,omitempty"`
	PreToolUse         []CursorHookEntry `json:"preToolUse,omitempty"`
	PostToolUse        []CursorHookEntry `json:"postToolUse,omitempty"`
}

// CursorHookEntry represents a single hook command.
// Cursor hooks have a command string and an optional matcher field for filtering by tool name.
//
//nolint:revive // CursorHookEntry is clearer than HookEntry when used outside this package
type CursorHookEntry struct {
	Command string `json:"command"`
	Matcher string `json:"matcher,omitempty"`
}

// sessionInfoRaw is the JSON structure from SessionStart/SessionEnd/Stop hooks.
// Cursor may provide session_id or conversation_id (fallback).
type sessionInfoRaw struct {
	// common
	ConversationID string   `json:"conversation_id"`
	GenerationID   string   `json:"generation_id"`
	Model          string   `json:"model"`
	HookEventName  string   `json:"hook_event_name"`
	CursorVersion  string   `json:"cursor_version"`
	WorkspaceRoots []string `json:"workspace_roots"`
	UserEmail      string   `json:"user_email"`
	TranscriptPath string   `json:"transcript_path"`
}

// beforeSubmitPromptInputRaw is the JSON structure from BeforeSubmitPrompt hooks.
type beforeSubmitPromptInputRaw struct {
	// common
	ConversationID string   `json:"conversation_id"`
	GenerationID   string   `json:"generation_id"`
	Model          string   `json:"model"`
	HookEventName  string   `json:"hook_event_name"`
	CursorVersion  string   `json:"cursor_version"`
	WorkspaceRoots []string `json:"workspace_roots"`
	UserEmail      string   `json:"user_email"`
	TranscriptPath string   `json:"transcript_path"`

	// hook specific
	Prompt string `json:"prompt"`
}

// preToolUseHookInputRaw is the JSON structure from PreToolUse[Task] hook.
type preToolUseHookInputRaw struct {
	// common
	ConversationID string   `json:"conversation_id"`
	GenerationID   string   `json:"generation_id"`
	Model          string   `json:"model"`
	HookEventName  string   `json:"hook_event_name"`
	CursorVersion  string   `json:"cursor_version"`
	WorkspaceRoots []string `json:"workspace_roots"`
	UserEmail      string   `json:"user_email"`
	TranscriptPath string   `json:"transcript_path"`

	// hook specific
	ToolUseID string          `json:"tool_use_id"`
	ToolInput json.RawMessage `json:"tool_input"`
	ToolName  string          `json:"tool_name"`
}

// postToolUseHookInputRaw is the JSON structure from PostToolUse hooks.
type postToolUseHookInputRaw struct {
	// common
	ConversationID string   `json:"conversation_id"`
	GenerationID   string   `json:"generation_id"`
	Model          string   `json:"model"`
	HookEventName  string   `json:"hook_event_name"`
	CursorVersion  string   `json:"cursor_version"`
	WorkspaceRoots []string `json:"workspace_roots"`
	UserEmail      string   `json:"user_email"`
	TranscriptPath string   `json:"transcript_path"`

	// hook specific
	ToolName   string          `json:"tool_name"`
	ToolInput  json.RawMessage `json:"tool_input"`
	ToolOutput string          `json:"tool_output"`
	ToolUseID  string          `json:"tool_use_id"`
	Cwd        string          `json:"cwd"`
}
