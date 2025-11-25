// main.go
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
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
	}{dir, idx, model, hash, time.Now().Format(time.RFC3339)})
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

// ── Model Call (stub) ───────────────────────────
func Call(model, prompt string) string {
	time.Sleep(200 * time.Millisecond)
	short := prompt
	if len(short) > 140 {
		short = short[:140] + "…"
	}
	return fmt.Sprintf("[%s]\n%s", model, short)
}

// ── Topologies ───────────────────────────────────
func standardChain(prompt string, models []string) (string, []Layer) {
	l := &PacketHeader{OriginalPrompt: prompt}
	var stack []Layer
	for i, m := range models {
		p := l.Build() + "\n\n→ " + m + "\nImprove the reasoning."
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
			p := l.Build() + "\n\n[Forward] You are " + m
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
			p := l.Build() + "\n\n[Reverse] You are " + m
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
	return fmt.Sprintf("Star Topology — %d arms × %d steps", len(models), steps), stack
}

// ── HTTP Handler ─────────────────────────────────
func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST only",http.StatusMethodNotAllowed)
		return
	}
	var in req
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if len(in.Models) == 0 {
		http.Error(w, "no models", 400)
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
	fmt.Println("MAIRE backend is LIVE")
	fmt.Println("→ http://localhost:8080/maire/run")
	fmt.Println("Topologies ready: standard-chain | double-helix | star-topology")
	fmt.Println("")
	log.Fatal(http.ListenAndServe(":8080", nil))
}