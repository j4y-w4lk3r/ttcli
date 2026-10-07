package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Rewrite asks the model for a new title and plain-text notes.
// A field the model omits stays as it was.
func Rewrite(ctx context.Context, cfg Config, title, notes, instruction string) (string, string, error) {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return "", "", fmt.Errorf("say what should change")
	}
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return "", "", fmt.Errorf("AI model is not configured")
	}
	content, err := complete(ctx, cfg, title, notes, instruction, true)
	if err != nil && strings.Contains(err.Error(), "HTTP 400") {
		content, err = complete(ctx, cfg, title, notes, instruction, false)
	}
	if err != nil {
		return "", "", err
	}
	nextTitle, nextNotes, err := parseProposal(content)
	if err != nil {
		return "", "", err
	}
	if nextTitle == nil {
		titleCopy := title
		nextTitle = &titleCopy
	}
	if strings.TrimSpace(*nextTitle) == "" {
		return "", "", fmt.Errorf("the model returned an empty title")
	}
	if nextNotes == nil {
		notesCopy := notes
		nextNotes = &notesCopy
	}
	return *nextTitle, *nextNotes, nil
}

func complete(ctx context.Context, cfg Config, title, notes, instruction string, jsonMode bool) (string, error) {
	payload := map[string]any{
		"model":       cfg.Model,
		"temperature": 0.2,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt(title, notes, instruction)},
		},
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI request failed: HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("AI response was not JSON")
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("AI response was empty")
	}
	return parsed.Choices[0].Message.Content, nil
}

const systemPrompt = `You edit one task. Reply with a JSON object {"title":"...","notes":"..."}.
Notes are plain text. Copy a field unchanged when the instruction does not ask to change it.
Do not add dates, priority, or checklist items.`

func userPrompt(title, notes, instruction string) string {
	if strings.TrimSpace(notes) == "" {
		notes = "(no notes)"
	}
	return "Title:\n" + title + "\n\nNotes:\n" + notes + "\n\nInstruction:\n" + instruction
}

func parseProposal(content string) (title, notes *string, err error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var proposal struct {
		Title *string `json:"title"`
		Notes *string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(content), &proposal); err != nil {
		return nil, nil, fmt.Errorf("AI response was not a title and notes")
	}
	return proposal.Title, proposal.Notes, nil
}
