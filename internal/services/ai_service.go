package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type AIService interface {
	GenerateArticleContent(title string) (string, error)
}

type aiService struct {
	apiKey         string
	baseUrl        string
	model          string
	fallbackModels []string
}

func NewAIService() AIService {
	return &aiService{
		apiKey:  os.Getenv("TOGETHER_AI_KEY"),
		baseUrl: "https://api.together.xyz/v1",
		model:   "google/gemma-4-31B-it",
		fallbackModels: []string{
			"meta-llama/Llama-3-8b-chat-hf",
			"google/gemma-2-9b-it",
			"mistralai/Mistral-7B-Instruct-v0.2",
			"Qwen/Qwen2-7B-Instruct",
		},
	}
}

func (s *aiService) GenerateArticleContent(title string) (string, error) {
	return s.generateWithFallback(title, 0)
}

func (s *aiService) generateWithFallback(title string, attempt int) (string, error) {
	if s.apiKey == "" {
		return "", errors.New("Together AI API Key is missing")
	}

	isArabicTitle := regexp.MustCompile(`[\x{0600}-\x{06FF}]`).MatchString(title)

	var systemPrompt, userPrompt string
	if isArabicTitle {
		systemPrompt = "أنت كاتب عربي محترف من الأردن. اكتب نصاً عربياً فقط بدون أي مقدمات أو تعليقات أو عناوين."
		userPrompt = fmt.Sprintf("اكتب محتوى مقال احترافي عن العنوان التالي: \"%s\". طول النص من 6 إلى 10 أسطر. لا تكتب العنوان، اكتب نص المقال فقط. ممنوع كتابة أي جملة تمهيدية مثل: \"هذا مسودة...\" أو \"إليك\".", title)
	} else {
		systemPrompt = "You are a professional writer. Write only the article body without any preface or labels."
		userPrompt = fmt.Sprintf("Write a professional article about: \"%s\". The content should be 6 to 10 lines. Do not include the title, only the body.", title)
	}

	var currentModel string
	if attempt == 0 {
		currentModel = s.model
	} else if attempt-1 < len(s.fallbackModels) {
		currentModel = s.fallbackModels[attempt-1]
	} else {
		return "", errors.New("failed to generate content: All AI models unavailable")
	}

	payload := map[string]interface{}{
		"model": currentModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":         512,
		"temperature":        0.7,
		"top_p":              0.7,
		"top_k":              50,
		"repetition_penalty": 1,
		"stop":               []string{"<|eot_id|>"},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", MapError(err)
	}

	req, err := http.NewRequest("POST", s.baseUrl+"/chat/completions", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", MapError(err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if attempt < len(s.fallbackModels) {
			log.Printf("Attempting fallback model: %s", s.fallbackModels[attempt])
			return s.generateWithFallback(title, attempt+1)
		}
		return "", fmt.Errorf("failed to generate content: %v", MapError(err))
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", MapError(err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &errorData); err == nil {
			if errMap, ok := errorData["error"].(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					log.Printf("Together AI API Error (model: %s): %s", currentModel, msg)
					if attempt < len(s.fallbackModels) {
						log.Printf("Attempting fallback model: %s", s.fallbackModels[attempt])
						return s.generateWithFallback(title, attempt+1)
					}
					return "", fmt.Errorf("failed to generate content: %s", msg)
				}
			}
		}
		if attempt < len(s.fallbackModels) {
			log.Printf("Attempting fallback model: %s", s.fallbackModels[attempt])
			return s.generateWithFallback(title, attempt+1)
		}
		return "", fmt.Errorf("failed to generate content: %s", string(bodyBytes))
	}

	var data struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return "", MapError(err)
	}

	if len(data.Choices) > 0 {
		content := strings.TrimSpace(data.Choices[0].Message.Content)
		if isArabicTitle {
			// Strip out English prefixes if they appear before Arabic text
			arabicMatch := regexp.MustCompile(`[\x{0600}-\x{06FF}]`).FindStringIndex(content)
			if arabicMatch != nil {
				content = content[arabicMatch[0]:]
			}
		}
		return content, nil
	}

	return "", errors.New("no content generated")
}
