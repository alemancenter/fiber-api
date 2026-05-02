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
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("title is required")
	}

	return s.generateWithFallback(title, 0)
}

func (s *aiService) generateWithFallback(title string, attempt int) (string, error) {
	if s.apiKey == "" {
		return "", errors.New("Together AI API Key is missing")
	}

	currentModel, err := s.resolveModel(attempt)
	if err != nil {
		return "", err
	}

	isArabicTitle := containsArabic(title)
	systemPrompt, userPrompt := buildPrompts(title, isArabicTitle)

	payload := map[string]interface{}{
		"model": currentModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},

		// Better for 450–600 Arabic words
		"max_tokens": 1400,

		// More creative, less duplicated
		"temperature":        0.85,
		"top_p":              0.9,
		"top_k":              80,
		"repetition_penalty": 1.15,

		"stop": []string{
			"<|eot_id|>",
			"###",
			"```",
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", MapError(err)
	}

	req, err := http.NewRequest("POST", s.baseUrl+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", MapError(err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 90 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return s.tryFallbackOrError(title, attempt, fmt.Errorf("failed to call Together AI: %w", MapError(err)))
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", MapError(err)
	}

	if resp.StatusCode != http.StatusOK {
		apiErr := extractAPIError(responseBytes)
		if apiErr == "" {
			apiErr = string(responseBytes)
		}

		log.Printf("Together AI API Error (model: %s): %s", currentModel, apiErr)
		return s.tryFallbackOrError(title, attempt, fmt.Errorf("failed to generate content: %s", apiErr))
	}

	content, err := parseAIContent(responseBytes)
	if err != nil {
		return s.tryFallbackOrError(title, attempt, err)
	}

	content = cleanAIContent(content, isArabicTitle)

	if err := validateGeneratedContent(content, isArabicTitle); err != nil {
		log.Printf("AI generated weak content (model: %s): %v", currentModel, err)
		return s.tryFallbackOrError(title, attempt, err)
	}

	return content, nil
}

func (s *aiService) resolveModel(attempt int) (string, error) {
	if attempt == 0 {
		return s.model, nil
	}

	index := attempt - 1
	if index >= 0 && index < len(s.fallbackModels) {
		return s.fallbackModels[index], nil
	}

	return "", errors.New("failed to generate content: all AI models unavailable")
}

func (s *aiService) tryFallbackOrError(title string, attempt int, err error) (string, error) {
	if attempt < len(s.fallbackModels) {
		log.Printf("Attempting fallback model: %s", s.fallbackModels[attempt])
		return s.generateWithFallback(title, attempt+1)
	}

	return "", err
}

func buildPrompts(title string, isArabic bool) (string, string) {
	if isArabic {
		systemPrompt := `
أنت كاتب محتوى عربي محترف متخصص في المحتوى التعليمي.
اكتب بلغة عربية سليمة وواضحة ومناسبة للطلاب وأولياء الأمور.
المحتوى يجب أن يكون أصليًا بالكامل، غير مكرر، ومفيدًا للقارئ.
ممنوع كتابة العنوان.
ممنوع كتابة أي مقدمة خارج النص مثل: "إليك المقال" أو "هذا نص".
ممنوع استخدام عبارات عامة مستهلكة مثل: "في عالمنا اليوم"، "لا شك أن"، "مما لا شك فيه".
ممنوع نسخ أو إعادة صياغة نصوص محفوظة أو قوالب جاهزة.
ممنوع إضافة روابط.
ممنوع اختراع أرقام أو إحصاءات أو معلومات غير مؤكدة.
ممنوع استخدام خاتمة مكررة مثل: "وفي الختام" أو "في النهاية".
اكتب النص فقط بدون أي شرح إضافي.
`

		userPrompt := fmt.Sprintf(`
اكتب محتوى مقال تعليمي احترافي عن العنوان التالي: "%s".

الشروط الإلزامية:
- الطول بين 450 و 600 كلمة.
- اللغة العربية فقط.
- لا تكتب العنوان.
- لا تكتب عناوين فرعية إلا إذا كانت ضرورية جدًا.
- ابدأ مباشرة بفكرة قوية مرتبطة بالعنوان.
- قسم النص إلى فقرات قصيرة وواضحة.
- اجعل كل فقرة تضيف فكرة جديدة.
- لا تكرر نفس المعنى بصياغات مختلفة.
- استخدم أسلوبًا تعليميًا عمليًا يناسب موقعًا تعليميًا عربيًا.
- اجعل النص مفيدًا للطالب أو ولي الأمر أو المعلم.
- تجنب الحشو والجمل العامة.
- لا تضف روابط.
- لا تستخدم خاتمة تقليدية.
- لا تذكر أنك ذكاء اصطناعي.
- لا تستخدم كلمات مثل: "سأكتب"، "إليك"، "في هذا المقال".

اكتب النص النهائي فقط.
`, title)

		return strings.TrimSpace(systemPrompt), strings.TrimSpace(userPrompt)
	}

	systemPrompt := `
You are a professional educational content writer.
Write original, useful, non-repetitive educational content.
Do not include titles, labels, explanations, or prefaces.
Avoid generic filler phrases.
Do not invent statistics.
Do not add links.
Write only the final article body.
`

	userPrompt := fmt.Sprintf(`
Write a professional educational article about: "%s".

Requirements:
- 450 to 600 words.
- English only.
- Do not include the title.
- Start directly with useful information.
- Use short, connected paragraphs.
- Each paragraph must add a new idea.
- Avoid repeated wording and generic phrases.
- Do not add links.
- Do not invent statistics.
- Do not write a generic conclusion.
- Do not mention AI.
- Write only the final article body.
`, title)

	return strings.TrimSpace(systemPrompt), strings.TrimSpace(userPrompt)
}

func parseAIContent(bodyBytes []byte) (string, error) {
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

	if len(data.Choices) == 0 {
		return "", errors.New("no content generated")
	}

	content := strings.TrimSpace(data.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("empty content generated")
	}

	return content, nil
}

func extractAPIError(bodyBytes []byte) string {
	var errorData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &errorData); err != nil {
		return ""
	}

	errValue, ok := errorData["error"]
	if !ok {
		return ""
	}

	switch v := errValue.(type) {
	case string:
		return v
	case map[string]interface{}:
		if msg, ok := v["message"].(string); ok {
			return msg
		}
	}

	return ""
}

func cleanAIContent(content string, isArabic bool) string {
	content = strings.TrimSpace(content)

	// Remove markdown fences
	content = strings.ReplaceAll(content, "```html", "")
	content = strings.ReplaceAll(content, "```text", "")
	content = strings.ReplaceAll(content, "```", "")

	// Remove common labels
	prefixes := []string{
		"المقال:",
		"النص:",
		"المحتوى:",
		"محتوى المقال:",
		"إليك المقال:",
		"إليك النص:",
		"Article:",
		"Content:",
		"Here is the article:",
		"Here is the content:",
	}

	for _, p := range prefixes {
		content = strings.TrimSpace(strings.TrimPrefix(content, p))
	}

	if isArabic {
		// If model adds English before Arabic, strip before first Arabic char
		arabicMatch := regexp.MustCompile(`[\x{0600}-\x{06FF}]`).FindStringIndex(content)
		if arabicMatch != nil {
			content = content[arabicMatch[0]:]
		}

		// Remove unwanted Arabic intros
		unwantedStarts := []string{
			"إليك مقال",
			"إليك النص",
			"هذا المقال",
			"في هذا المقال",
			"سأكتب",
		}

		for _, start := range unwantedStarts {
			if strings.HasPrefix(content, start) {
				content = strings.TrimSpace(strings.TrimPrefix(content, start))
			}
		}
	}

	// Normalize spaces and empty lines
	content = regexp.MustCompile(`[ \t]+`).ReplaceAllString(content, " ")
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")

	return strings.TrimSpace(content)
}

func validateGeneratedContent(content string, isArabic bool) error {
	plain := strings.TrimSpace(stripHTML(content))
	if plain == "" {
		return errors.New("generated content is empty")
	}

	wordCount := countWords(plain)

	// We requested 450–600 words, but allow slight under-generation by small models.
	if wordCount < 300 {
		return fmt.Errorf("generated content is too short: %d words", wordCount)
	}

	if wordCount > 750 {
		return fmt.Errorf("generated content is too long: %d words", wordCount)
	}

	if isArabic && !containsArabic(plain) {
		return errors.New("generated content is not Arabic")
	}

	if hasBadAIIntro(plain) {
		return errors.New("generated content contains unwanted AI intro")
	}

	if hasExcessiveRepetition(plain) {
		return errors.New("generated content contains excessive repetition")
	}

	return nil
}

func stripHTML(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}

func countWords(s string) int {
	words := strings.Fields(s)
	return len(words)
}

func containsArabic(s string) bool {
	return regexp.MustCompile(`[\x{0600}-\x{06FF}]`).MatchString(s)
}

func hasBadAIIntro(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))

	badStarts := []string{
		"إليك",
		"هذا المقال",
		"في هذا المقال",
		"سأكتب",
		"بالطبع",
		"here is",
		"this article",
		"in this article",
		"of course",
	}

	for _, start := range badStarts {
		if strings.HasPrefix(lower, strings.ToLower(start)) {
			return true
		}
	}

	return false
}

func hasExcessiveRepetition(s string) bool {
	normalized := strings.ToLower(s)
	normalized = regexp.MustCompile(`[^\p{L}\p{N}\s]+`).ReplaceAllString(normalized, "")
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

	words := strings.Fields(normalized)
	if len(words) < 80 {
		return false
	}

	phraseCount := make(map[string]int)

	// Detect repeated 5-word phrases
	for i := 0; i+5 <= len(words); i++ {
		phrase := strings.Join(words[i:i+5], " ")
		phraseCount[phrase]++

		if phraseCount[phrase] >= 3 {
			return true
		}
	}

	return false
}
