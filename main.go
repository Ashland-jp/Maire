package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Types
type Layer struct {
	Model    string `json:"model"`
	Response string `json:"response"`
}

type PacketHeader struct {
	Ledger []struct {
		Index int    `json:"index"`
		Model string `json:"model"`
		Hash  string `json:"hash"`
	} `json:"ledger"`
	Original string `json:"original"`
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

// Real LLM calls — FIXED routing
func Call(modelId, prompt string) string {
	// Map UI model IDs to real backends
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
	// Fallback stub if key missing
	return fmt.Sprintf("[STUB %s]\n%s", modelId, prompt[:100]+"...")
}

// --- LLM CALLERS (unchanged but confirmed working) ---
func callOpenRouter(prompt, key string) string {
	body := map[string]any{
		"model":    "meta-llama/llama-3.1-8b-instruct",
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "http://localhost:5173")
	req.Header.Set("X-Title", "MAIRE")
	client := &http.Client{Timeout: 90 * time.Second}
	res, err := client.Do(req)
	if err != nil { return "[OpenRouter error]" }
	defer res.Body.Close()
	data := map[string]any{}
	json.NewDecoder(res.Body).Decode(&data)
	if choices, ok := data["choices"].([]any); ok && len(choices) > 0 {
		if msg, ok := choices[0].(map[string]any)["message"].(map[string]any); ok {
			return msg["content"].(string)
		}
	}
	return "[OpenRouter no content]"
}

func callGemini(prompt, key string) string {
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + key
	body := map[string]any{"contents": []map[string]any{{"parts": []map[string]string{{"text": prompt}}}}}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 90 * time.Second}
	res, err := client.Do(req)
	if err != nil { return "[Gemini error]" }
	defer res.Body.Close()
	var data struct {
		Candidates []struct {
			Content struct {
				Parts []struct{ Text string } `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	json.NewDecoder(res.Body).Decode(&data)
	if len(data.Candidates) > 0 && len(data.Candidates[0].Content.Parts) > 0 {
		return data.Candidates[0].Content.Parts[0].Text
	}
	return "[Gemini no content]"
}

func callHuggingFace(prompt, key string) string {
	url := "https://api-inference.huggingface.co/models/mistralai/Mixtral-8x7B-Instruct-v0.1"
	body := map[string]any{"inputs": prompt, "parameters": map[string]int{"max_new_tokens": 1024}}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	res, err := client.Do(req)
	if err != nil { return "[HF error]" }
	defer res.Body.Close()
	var result []map[string]string
	json.NewDecoder(res.Body).Decode(&result)
	if len(result) > 0 && result[0]["generated_text"] != "" {
		return result[0]["generated_text"]
	}
	return "[HF no content]"
}

// TRUE STAR TOPOLOGY — permuted sequential chains
func trueStarTopology(originalPrompt string, models []string) (string, []Layer) {
	var stack []Layer

	// Generate all cyclic permutations (each model starts exactly once)
	n := len(models)
	for start := 0; start < n; start++ {
		chain := make([]string, n)
		for i := 0; i < n; i++ {
			chain[i] = models[(start+i)%n]
		}

		ledger := ""
		currentPrompt := originalPrompt

		for step, model := range chain {
			systemPrompt := fmt.Sprintf(`<LEDGER>%s</LEDGER>

		Please answer the original question, 
		short and concise, including whether you agree
		 or not with the previous responses on the ledger, if any.`, ledger)

			response := Call(model, systemPrompt+"\n\nUser: "+currentPrompt)
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(response)))[:8]

			ledger += fmt.Sprintf("\nStep %d → %s\nHash: %s\n%s", step+1, model, hash, response)

			stack = append(stack, Layer{
				Model:    fmt.Sprintf("Model %d (%s → %s)", start+1, chain[0], chain[n-1]),
				Response: response,
			})

			currentPrompt = response // pass forward
		}

		// Add separator between chains
		if start < n-1 {
			stack = append(stack, Layer{Model: "─", Response: ""})
		}
	}

	return fmt.Sprintf("True Star Topology Complete — %d chains", len(models)), stack
}

// Handler
func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/maire/run" {
		http.Error(w, "not found", 404)
		return
	}
	var in req
	json.NewDecoder(r.Body).Decode(&in)

	var final string
	var stack []Layer

	if in.Topology == "star-topology" {
		final, stack = trueStarTopology(in.OriginalPrompt, in.Models)
	} else {
		// fallback
		final = "Topology not implemented yet"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp{FinalResponse: final, HeaderStack: stack})
}

func modelsHandler(w http.ResponseWriter, r *http.Request) {
	avail := []map[string]string{}
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		avail = append(avail, map[string]string{"id": "grok", "name": "Grok (Llama 3.1 via OpenRouter)"})
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		avail = append(avail, map[string]string{"id": "gpt-4", "name": "GPT-4 (Gemini 1.5 Flash)"})
	}
	if os.Getenv("HF_API_KEY") != "" {
		avail = append(avail, map[string]string{"id": "claude", "name": "Claude (Mixtral 8x7B)"})
	}
	json.NewEncoder(w).Encode(avail)
}

func main() {
	http.HandleFunc("/maire/run", handler)
	http.HandleFunc("/maire/models", modelsHandler)
	fmt.Println("MAIRE TRUE STAR LIVE → http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}