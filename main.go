package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type Layer struct {
	Model    string `json:"model"`
	Response string `json:"response"`
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

// === REAL LLM CALLS (FIXED) ===
func Call(modelId, prompt string) string {
	switch modelId {
	case "grok":
		if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
			return callOpenRouter(prompt, key)
		}
	case "gpt-4":
		if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
			return callGemini(prompt, key)
		}
	case "claude":
		if key := os.Getenv("HF_API_KEY"); key != "" {
			return callHuggingFace(prompt, key)
		}
	}
	return fmt.Sprintf("[LOCAL %s]\n%s", modelId, prompt[:120]+"...")
}

// OpenRouter → Llama 3.1
func callOpenRouter(prompt, key string) string {
	payload := map[string]any{
		"model": "meta-llama/llama-3.1-8b-instruct",
		"messages": []map[string]string{
			{"role": "system", "content": "You are a reasoning engine. Never mention your name or model."},
			{"role": "user", "content": prompt},
		},
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "http://localhost:5173")
	req.Header.Set("X-Title", "MAIRE")
	client := &http.Client{Timeout: 90 * time.Second}
	res, err := client.Do(req)
	if err != nil { return "[OpenRouter error]" }
	defer res.Body.Close()
	var data struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(res.Body).Decode(&data) == nil && len(data.Choices) > 0 {
		return data.Choices[0].Message.Content
	}
	return "[OpenRouter empty]"
}

// Gemini 1.5 Flash
func callGemini(prompt, key string) string {
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + key
	payload := map[string]any{"contents": []map[string]any{{"parts": []map[string]string{{"text": prompt}}}}}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 90 * time.Second}
	res, err := client.Do(req)
	if err != nil { return "[Gemini error]" }
	defer res.Body.Close()
	var data struct {
		Candidates []struct {
			Content struct{ Parts []struct{ Text string } } `json:"content"`
		} `json:"candidates"`
	}
	if json.NewDecoder(res.Body).Decode(&data) == nil && len(data.Candidates) > 0 && len(data.Candidates[0].Content.Parts) > 0 {
		return data.Candidates[0].Content.Parts[0].Text
	}
	return "[Gemini empty]"
}

// Hugging Face → Mixtral
func callHuggingFace(prompt, key string) string {
	url := "https://api-inference.huggingface.co/models/mistralai/Mixtral-8x7B-Instruct-v0.1"
	payload := map[string]any{
		"inputs": prompt,
		"parameters": map[string]any{"max_new_tokens": 1024},
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	res, err := client.Do(req)
	if err != nil { return "[HF error]" }
	defer res.Body.Close()
	var result []map[string]string
	if json.NewDecoder(res.Body).Decode(&result) == nil && len(result) > 0 {
		return result[0]["generated_text"]
	}
	return "[HF empty]"
}

// === HELIX CHAIN (forward + reverse) ===
func runHelixChain(originalPrompt string, order []string, stack *[]Layer) string {
	ledger := ""
	current := originalPrompt

	for i, model := range order {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(current)))[:8]
		ledger += fmt.Sprintf("\nStep %d | %s | %s\n%s", i+1, model, hash, current)

		prompt := fmt.Sprintf(`<LEDGER>%s</LEDGER>
Disregard if there is no ledge. This ledger is strictly immutable and append only. Do not alter previous entries.
Given the reasoning trace above, provide your opinion or version of facts. you can agree, disagree or make corrections.
 Be clear and concise in your response.
Respond concisely.`, ledger)

		response := Call(model, prompt)
		*stack = append(*stack, Layer{Model: fmt.Sprintf("→ %s", model), Response: response})
		current = response
	}

	return current
}

// === TOPOLOGIES ===
func standardChain(prompt string, models []string) (string, []Layer) {
	var stack []Layer
	order := append(models, reverse(models[1:len(models)-1])...) // 1→2→3→2→1
	final := runHelixChain(prompt, order, &stack)
	return final, stack
}

func doubleHelix(prompt string, models []string) (string, []Layer) {
	var stack []Layer

	// Forward helix
	runHelixChain(prompt, append(models, reverse(models[1:len(models)-1])...), &stack)
	stack = append(stack, Layer{Model: "────────── REVERSE HELIX ──────────", Response: ""})

	// Reverse helix
	runHelixChain(prompt, append(reverse(models), models[1:len(models)-1]...), &stack)

	// Final summary by Model 1
	summaryPrompt := fmt.Sprintf("Summarize the full reasoning trace above into a final answer. Be concise and authoritative.")
	summary := Call(models[0], summaryPrompt)
	stack = append(stack, Layer{Model: "FINAL SUMMARY (Model 1)", Response: summary})

	return "Double Helix + Final Summary", stack
}

func starTopology(prompt string, models []string) (string, []Layer) {
	var stack []Layer
	n := len(models)

	for start := 0; start < n; start++ {
		chain := make([]string, 0, len(models)*2-1)
		// Forward from start
		for i := 0; i < n; i++ {
			chain = append(chain, models[(start+i)%n])
		}
		// Reverse (skip first and last to avoid double)
		rev := reverse(chain)
		chain = append(chain, rev[1:len(rev)-1]...)

		stack = append(stack, Layer{Model: fmt.Sprintf("STAR CHAIN %d (starts with %s)", start+1, models[start]), Response: ""})
		runHelixChain(prompt, chain, &stack)
		if start < n-1 {
			stack = append(stack, Layer{Model: "════════════════════════════════", Response: ""})
		}
	}

	// Final summary by Model 1
	summary := Call(models[0], "Provide a final authoritative answer based on all chains above.")
	stack = append(stack, Layer{Model: "FINAL STAR SUMMARY (Model 1)", Response: summary})

	return fmt.Sprintf("Star Topology — %d chains completed", n), stack
}

func reverse(s []string) []string {
	r := make([]string, len(s))
	for i, v := range s {
		r[len(s)-1-i] = v
	}
	return r
}

// === HTTP HANDLERS ===
func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/maire/run" {
		http.Error(w, "not found", 404)
		return
	}
	var in req
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		http.Error(w, "bad json", 400)
		return
	}

	var final string
	var stack []Layer

	switch in.Topology {
	case "standard-chain":
		final, stack = standardChain(in.OriginalPrompt, in.Models)
	case "double-helix":
		final, stack = doubleHelix(in.OriginalPrompt, in.Models)
	case "star-topology":
		final, stack = starTopology(in.OriginalPrompt, in.Models)
	default:
		final = "Unknown topology"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp{FinalResponse: final, HeaderStack: stack})
}

func modelsHandler(w http.ResponseWriter, r *http.Request) {
	avail := []map[string]string{}
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		avail = append(avail, map[string]string{"id": "grok", "name": "Grok (Llama 3.1)"})
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		avail = append(avail, map[string]string{"id": "gpt-4", "name": "GPT-4 (Gemini Flash)"})
	}
	if os.Getenv("HF_API_KEY") != "" {
		avail = append(avail, map[string]string{"id": "claude", "name": "Claude (Mixtral)"})
	}
	json.NewEncoder(w).Encode(avail)
}

func main() {
	http.HandleFunc("/maire/run", handler)
	http.HandleFunc("/maire/models", modelsHandler)
	fmt.Println("\nMAIRE v3 — TRUE STAR TOPOLOGY LIVE")
	fmt.Println("→ http://localhost:8080/maire/run")
	fmt.Println("→ http://localhost:8080/maire/models")
	log.Fatal(http.ListenAndServe(":8080", nil))
}