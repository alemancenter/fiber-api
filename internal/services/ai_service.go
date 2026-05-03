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
	baseURL        string
	model          string
	fallbackModels []string
	httpClient     *http.Client
}

func NewAIService() AIService {
	return &aiService{
		apiKey:  strings.TrimSpace(os.Getenv("TOGETHER_AI_KEY")),
		baseURL: "https://api.together.xyz/v1",
		model:   "google/gemma-4-31B-it",
		fallbackModels: []string{
			"zai-org/GLM-5.1",
			"google/gemma-3n-E4B-it",
			"MiniMaxAI/MiniMax-M2.7",
		},
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (s *aiService) GenerateArticleContent(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("title is required")
	}

	if s.apiKey == "" {
		return "", errors.New("Together AI API key is missing")
	}

	return s.generateWithFallback(title, 0)
}

func (s *aiService) generateWithFallback(title string, attempt int) (string, error) {
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
		"max_tokens":         1800,
		"temperature":        0.75,
		"top_p":              0.9,
		"repetition_penalty": 1.12,
		"stop": []string{
			"<|eot_id|>",
			"```",
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", MapError(err)
	}

	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", MapError(err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return s.tryFallbackOrError(title, attempt, fmt.Errorf("failed to call Together AI: %w", MapError(err)))
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", MapError(err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := extractAPIError(responseBytes)
		if apiErr == "" {
			apiErr = string(responseBytes)
		}

		log.Printf("Together AI API error | model=%s | status=%d | error=%s", currentModel, resp.StatusCode, apiErr)
		return s.tryFallbackOrError(title, attempt, fmt.Errorf("failed to generate content: %s", apiErr))
	}

	content, err := parseAIContent(responseBytes)
	if err != nil {
		return s.tryFallbackOrError(title, attempt, err)
	}

	content = cleanAIContent(content, isArabicTitle)

	if err := validateGeneratedContent(content, isArabicTitle); err != nil {
		log.Printf("Weak AI content | model=%s | error=%v", currentModel, err)
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
		log.Printf("Trying fallback model: %s", s.fallbackModels[attempt])
		return s.generateWithFallback(title, attempt+1)
	}

	return "", err
}

func buildPrompts(title string, isArabic bool) (string, string) {
	if isArabic {
		systemPrompt := `
أنت كاتب محتوى عربي محترف متخصص في المقالات التعليمية المتوافقة مع SEO.
اكتب محتوى أصليًا ومفيدًا بلغة عربية فصحى واضحة.
ركّز على نية البحث، وقيمة القارئ، وسهولة القراءة.
ممنوع كتابة العنوان الرئيسي.
ممنوع كتابة مقدمات خارجية مثل: "إليك المقال" أو "هذا نص".
ممنوع استخدام الحشو والعبارات المستهلكة.
ممنوع اختراع أرقام أو إحصاءات.
ممنوع إضافة روابط.
ممنوع ذكر أنك ذكاء اصطناعي.
اكتب النص النهائي فقط.
`

		userPrompt := fmt.Sprintf(`
اكتب مقال SEO تعليمي احترافي عن العنوان التالي: "%s".

الشروط الإلزامية:
- الطول بين 500 و 700 كلمة.
- اللغة العربية الفصحى فقط.
- لا تكتب العنوان الرئيسي.
- ابدأ مباشرة بمقدمة قوية مرتبطة بنية البحث.
- استخدم فقرات قصيرة وواضحة.
- أضف عناوين فرعية طبيعية عند الحاجة فقط.
- اجعل المقال مناسبًا للطالب وولي الأمر والمعلم.
- ركز على الفائدة العملية وليس الكلام الإنشائي.
- لا تكرر نفس الفكرة بصياغات مختلفة.
- لا تستخدم عبارات مثل: "مما لا شك فيه"، "في عالمنا اليوم"، "لا شك أن".
- لا تضف روابط.
- لا تخترع معلومات غير مؤكدة.
- لا تستخدم خاتمة تقليدية مثل: "وفي الختام".
- لا تبدأ بعبارات مثل: "إليك"، "في هذا المقال"، "سأكتب".

اكتب النص النهائي فقط.
`, title)

		return strings.TrimSpace(systemPrompt), strings.TrimSpace(userPrompt)
	}

	systemPrompt := `
You are a professional educational SEO content writer.
Write original, useful, non-repetitive educational content.
Do not include the main title.
Do not add links.
Do not invent statistics.
Do not mention AI.
Write only the final article body.
`

	userPrompt := fmt.Sprintf(`
Write a professional SEO educational article about: "%s".

Requirements:
- 500 to 700 words.
- English only.
- Do not include the main title.
- Start directly with useful information.
- Use short, clear paragraphs.
- Add natural subheadings only when useful.
- Avoid filler and repeated meanings.
- Do not add links.
- Do not invent statistics.
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
		if typ, ok := v["type"].(string); ok {
			return typ
		}
	}

	return ""
}

func cleanAIContent(content string, isArabic bool) string {
	content = strings.TrimSpace(content)

	content = strings.ReplaceAll(content, "```html", "")
	content = strings.ReplaceAll(content, "```text", "")
	content = strings.ReplaceAll(content, "```markdown", "")
	content = strings.ReplaceAll(content, "```", "")

	content = strings.ReplaceAll(content, "，", "،")
	content = strings.ReplaceAll(content, "。", ".")
	content = strings.ReplaceAll(content, "؛؛", "؛")
	content = strings.ReplaceAll(content, "،،", "،")

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
		arabicMatch := regexp.MustCompile(`[\x{0600}-\x{06FF}]`).FindStringIndex(content)
		if arabicMatch != nil {
			content = content[arabicMatch[0]:]
		}

		unwantedStarts := []string{
			"إليك مقال",
			"إليك النص",
			"هذا المقال",
			"في هذا المقال",
			"سأكتب",
			"بالطبع",
		}

		for _, start := range unwantedStarts {
			if strings.HasPrefix(content, start) {
				content = strings.TrimSpace(strings.TrimPrefix(content, start))
			}
		}
	}

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

	if wordCount < 450 {
		return fmt.Errorf("generated content is too short: %d words", wordCount)
	}

	if wordCount > 850 {
		return fmt.Errorf("generated content is too long: %d words", wordCount)
	}

	if isArabic && !containsArabic(plain) {
		return errors.New("generated content is not Arabic")
	}

	if isArabic && arabicRatio(plain) < 0.45 {
		return errors.New("generated content has low Arabic ratio")
	}

	if hasBadAIIntro(plain) {
		return errors.New("generated content contains unwanted AI intro")
	}

	if hasExcessiveRepetition(plain) {
		return errors.New("generated content contains excessive repetition")
	}

	if strings.HasSuffix(strings.TrimSpace(plain), "إن") {
		return errors.New("generated content appears incomplete")
	}

	return nil
}

func stripHTML(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func containsArabic(s string) bool {
	return regexp.MustCompile(`[\x{0600}-\x{06FF}]`).MatchString(s)
}

func arabicRatio(s string) float64 {
	letters := regexp.MustCompile(`\p{L}`).FindAllString(s, -1)
	if len(letters) == 0 {
		return 0
	}

	arabicLetters := regexp.MustCompile(`[\x{0600}-\x{06FF}]`).FindAllString(s, -1)
	return float64(len(arabicLetters)) / float64(len(letters))
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
	if len(words) < 100 {
		return false
	}

	phraseCount := make(map[string]int)

	for i := 0; i+5 <= len(words); i++ {
		phrase := strings.Join(words[i:i+5], " ")
		phraseCount[phrase]++

		if phraseCount[phrase] >= 3 {
			return true
		}
	}

	return false
}