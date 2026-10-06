package openai

import "testing"

func TestBuildChatGPTBodyTokenLimitField(t *testing.T) {
	limit := 300
	messages := []map[string]interface{}{{"role": "user", "content": "hi"}}

	body := BuildChatGPTBody("gpt-5.6-terra", messages, &limit, nil, false, OpenAIType)
	if _, ok := body["max_tokens"]; ok {
		t.Errorf("gpt-5 body must not send max_tokens: %v", body)
	}
	if body["max_completion_tokens"] != 300 {
		t.Errorf("gpt-5 body must send max_completion_tokens=300: %v", body)
	}

	body = BuildChatGPTBody("gpt-4.1-mini", messages, &limit, nil, false, OpenAIType)
	if body["max_tokens"] != 300 {
		t.Errorf("gpt-4.1-mini body must keep max_tokens=300: %v", body)
	}
	if _, ok := body["max_completion_tokens"]; ok {
		t.Errorf("gpt-4.1-mini body must not send max_completion_tokens: %v", body)
	}
}

func TestDefaultModelIsNotRetiring(t *testing.T) {
	if defaultModel != "gpt-4.1-mini" {
		t.Errorf("defaultModel = %q, want gpt-4.1-mini (gpt-3.5-turbo shuts down 23 Oct 2026)", defaultModel)
	}
}
