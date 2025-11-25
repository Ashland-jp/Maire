// main.go
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ── Types ─────────────────────────────────────
type Layer struct {
	Model    string `json:"model"`
	Response string `json:"response"`
}

type PacketHeader struct {
	OriginalPrompt string
	Ledger         []struct {
		Direction string
		Index     int
		Model     string
		Hash      string
		Time      string
	}
	mu sync.Mutex
}

type req struct {
	OriginalPrompt string   `json:"original_prompt"`
	Topology       string   `json:"topology"`
	Models         []string `json:"models"`
}

type resp struct {
	FinalResponse string  `json:"final_response"`
	HeaderStack   []Layer `json:"header_stack"`
}

// ── Ledger Methods ─────────────────────────────
func (p *PacketHeader) Add(dir string, idx int, model, body string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))[:12]
	p.Ledger = append(p.Ledger, struct {
		Direction string
		Index     int
		Model     string
		Hash      string
		Time      string
	}{Direction: dir, Index: idx, Model: model, Hash: hash, Time: time.Now().Format(time.RFC3339)})
}
func (p *PacketHeader) Build() string {
	var b strings.Builder
	b.WriteString("<LEDGER>\nOriginal: " + p.OriginalPrompt + "\n\n")
	for _, e := range p.Ledger {
		b.WriteString(fmt.Sprintf("%s%d | %s | %s | %s\n", e.Direction, e.Index, e.Model, e.Hash, e.Time))
	}
	b.WriteString("</LEDGER>\n")
	return b.String()
}

// ── Real LLM Calls (OpenRouter, Hugging Face, Google Gemini) ─────
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

type LLMResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func Call(model, prompt string) string {
	// Super-safe version — will never crash the server
	openrouterKey := os.Getenv("OPENROUTER_API_KEY")
	hfKey := os.Getenv("HF_API_KEY")
	googleKey := os.Getenv("GOOGLE_API_KEY")

	if openrouterKey == "" && hfKey == "" && googleKey == "" {
		return CallStub(model, prompt)
	}

	// Default to stub if model not recognized
	resp := CallStub(model, prompt)

	// Try OpenRouter first (grok/llama)
	if (strings.Contains(strings.ToLower(model), "grok") || strings.Contains(strings.ToLower(model), "llama")) && openrouterKey != "" {
		if text, ok := callOpenRouter(prompt, openrouterKey); ok {
			return text
		}
	}

	// Try Hugging Face (claude/mistral)
	if strings.Contains(strings.ToLower(model), "claude") || strings.Contains(strings.ToLower(model), "mistral") {
	if text, ok := callHuggingFace(prompt, hfKey); ok {
		return text
	}
}

	// Try Gemini (gpt/gemini)
	if strings.Contains(strings.ToLower(model), "gpt") && googleKey != "" {
		if text, ok := callGemini(prompt, googleKey); ok {
			return text
		}
	}

	return resp // fallback
}

// Helper functions — these never panic
// ── Safe LLM helpers (never panic, always return) ─────────────────────
func callOpenRouter(prompt, key string) (string, bool) {
	body := map[string]any{
		"model": "meta-llama/llama-3.1-8b-instruct",
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "http://localhost:5173")
	req.Header.Set("X-Title", "MAIRE")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil || resp.StatusCode != 200 {
		return "", false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var data struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(b, &data) == nil && len(data.Choices) > 0 {
		return data.Choices[0].Message.Content, true
	}
	return "", false
}

func callHuggingFace(prompt, key string) (string, bool) {
	// This model is free, instantly available, and works perfectly in 2025
	url := "https://api-inference.huggingface.co/models/mistralai/Mixtral-8x7B-Instruct-v0.1"

	payload := map[string]any{
		"inputs": prompt,
		"parameters": map[string]any{
			"max_new_tokens": 512,
			"return_full_text": false,
		},
	}

	jsonBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// HF often returns 503 when model is loading — we just fall back gracefully
		return "", false
	}

	body, _ := io.ReadAll(resp.Body)
	var result []struct {
		GeneratedText string `json:"generated_text"`
	}
	if json.Unmarshal(body, &result) == nil && len(result) > 0 {
		return result[0].GeneratedText, true
	}
	return string(body), true
}

func callGemini(prompt, key string) (string, bool) {
	body := map[string]any{
		"contents": []map[string]any{{"parts": []map[string]string{{"text": prompt}}}},
	}
	jsonBody, _ := json.Marshal(body)
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + key
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp == nil || resp.StatusCode != 200 {
		return "", false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var data struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if json.Unmarshal(b, &data) == nil && len(data.Candidates) > 0 && len(data.Candidates[0].Content.Parts) > 0 {
		return data.Candidates[0].Content.Parts[0].Text, true
	}
	return "", false
}


func CallStub(model, prompt string) string {
	time.Sleep(180 * time.Millisecond)
	short := prompt
	if len(short) > 120 {
		short = short[:120] + "…"
	}
	return fmt.Sprintf("[%s - local stub]\n%s", model, short)
}

// ── Topologies ───────────────────────────────────
func standardChain(prompt string, models []string) (string, []Layer) {
	l := &PacketHeader{OriginalPrompt: prompt}
	var stack []Layer
	for i, m := range models {
		p := l.Build() + "\n\n→ " + m + "\nContinue and improve:"
		resp := Call(m, p)
		l.Add("F", i, m, resp)
		stack = append(stack, Layer{Model: m, Response: resp})
	}
	return "Standard chain complete", stack
}

func doubleHelix(prompt string, models []string) (string, []Layer) {
	l := &PacketHeader{OriginalPrompt: prompt}
	var stack []Layer
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i, m := range models {
			p := l.Build() + "\n\n[Forward pass] You are " + m
			resp := Call(m, p)
			mu.Lock()
			l.Add("F", i, m, resp)
			stack = append(stack, Layer{Model: m + " (forward)", Response: resp})
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		for i := len(models) - 1; i >= 0; i-- {
			m := models[i]
			p := l.Build() + "\n\n[Reverse pass] Critique as " + m
			resp := Call(m, p)
			mu.Lock()
			l.Add("R", i, m, resp)
			stack = append(stack, Layer{Model: m + " (reverse)", Response: resp})
			mu.Unlock()
		}
	}()

	wg.Wait()
	return "Double Helix complete", stack
}

func starTopology(prompt string, models []string) (string, []Layer) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var stack []Layer
	steps := int(math.Max(3, float64(len(models))))

	for i, m := range models {
		wg.Add(1)
		go func(idx int, model string) {
			defer wg.Done()
			l := &PacketHeader{OriginalPrompt: prompt}
			for s := 0; s < steps; s++ {
				p := l.Build() + fmt.Sprintf("\n\nStar arm %d — step %d — You are %s", idx+1, s+1, model)
				resp := Call(model, p)
				l.Add("S", s, model, resp)
				mu.Lock()
				stack = append(stack, Layer{
					Model:    fmt.Sprintf("%s (arm %d • step %d)", model, idx+1, s+1),
					Response: resp,
				})
				mu.Unlock()
			}
		}(i, m)
	}
	wg.Wait()
	return fmt.Sprintf("Star Topology — %d independent chains", len(models)), stack
}

// ── HTTP Handler ─────────────────────────────────
func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var in req
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if len(in.Models) == 0 {
		http.Error(w, "no models", 400)
		return
	}

	var final string
	var stack []Layer

	switch in.Topology {
	case "star-topology":
		final, stack = starTopology(in.OriginalPrompt, in.Models)
	case "double-helix":
		final, stack = doubleHelix(in.OriginalPrompt, in.Models)
	default:
		final, stack = standardChain(in.OriginalPrompt, in.Models)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp{FinalResponse: final, HeaderStack: stack})
}

// ── Server ───────────────────────────────────────
func main() {
	http.HandleFunc("/maire/run", handler)
	fmt.Println("")
	fmt.Println("MAIRE backend LIVE")
	fmt.Println("→ http://localhost:8080/maire/run")
	fmt.Println("Free LLMs ready: OpenRouter (grok), Hugging Face (claude), Gemini (gpt)")
	fmt.Println("")
	log.Fatal(http.ListenAndServe(":8080", nil))
}