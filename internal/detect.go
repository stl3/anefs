// Package detect extracts character names from ASS subtitle files using
// deterministic signals: honorific suffixes, vocative position, sentence
// structure, English word filtering, and alias normalisation.
package detect

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Candidate tracks a potential name and its accumulated evidence.
type Candidate struct {
	Name       string
	Score      int
	Count      int
	Honourific bool // true if ever scored via honourific
}

// Result is a final scored output item.
type Result struct {
	Name  string
	Score int
	Count int
}

// coGraph tracks co-occurrence pairs between candidate names within the same
// dialogue line. Names that co-occur frequently are more likely to be real
// characters; isolated names are more likely noise.
type coGraph struct {
	edges map[string]map[string]int
}

func newCoGraph() *coGraph {
	return &coGraph{edges: make(map[string]map[string]int)}
}

// add records a co-occurrence between two names.
func (g *coGraph) add(a, b string) {
	if a == b {
		return
	}
	if g.edges[a] == nil {
		g.edges[a] = make(map[string]int)
	}
	if g.edges[b] == nil {
		g.edges[b] = make(map[string]int)
	}
	g.edges[a][b]++
	g.edges[b][a]++
}

// partners returns the number of unique partners name has co-occurred with.
func (g *coGraph) partners(name string) int {
	return len(g.edges[name])
}

// Scoring weights. Higher = stronger evidence.
const (
	wHonorific   = 20
	wVocative    = 15
	wMidSentence = 3
	wSpeechVerb  = 5 // name before said/told/replied
	wPortmanteau = 5 // decomposed from compound like "MaiAji"
	wAllCapsName = 3 // all-caps name (e.g. "SENA:" overlay)
	wInQuotes    = 2
	wLongName    = 2 // name length >= 5
	wJapanese    = 2 // sounds Japanese
	pCaps        = 5 // penalty for ALL CAPS (subtracted)
)

// Thresholds.
const (
	minScore = 15
	minCount = 1
	minLen   = 2
)

// Regex patterns (compiled once at init).
var (
	// HonourificRx matches "Name-san", "Name-chan", etc.
	honourificRx = regexp.MustCompile(`\b([A-Z][a-z]+)-(san|chan|kun|senpai|sama|sensei|tan|bo|chin)\b`)

	// VocativeRx matches ", Name" at the end of a clause.
	vocativeRx = regexp.MustCompile(`[,!?]\s+([A-Z][a-z]+(?:-(?:san|chan|kun|senpai|sama|sensei))?)`)

	// ProperRx matches any capitalized word with 2+ lowercase letters.
	properRx = regexp.MustCompile(`\b[A-Z][a-z]{2,}\b`)

	// SpeechVerbRx matches "Name said", "Name replied", etc.
	speechVerbRx = regexp.MustCompile(
		`\b([A-Z][a-z]+(?:-(?:san|chan|kun|senpai|sama|sensei))?)\s+(said|says|replied|replies|asked|asks|answered|answers|whispered|whispers|shouted|shouts|yelled|yells|screamed|screams|muttered|mutters|called|calls|responded|responds|added|adds|exclaimed|exclaims|noted|notes)\b`)

	// AllCapsRx matches ALL CAPS words (2+ uppercase letters).
	allCapsRx = regexp.MustCompile(`\b[A-Z]{2,}\b`)

	// SentenceRx splits text into sentences on terminal punctuation.
	sentenceRx = regexp.MustCompile(`[.!?]+`)
)

// FromFile opens an ASS subtitle file and returns ranked name candidates.
func FromFile(path string) ([]Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	candidates := make(map[string]*Candidate)
	graph := newCoGraph()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := extractDialogue(scanner.Text())
		if text == "" {
			continue
		}
		text = expandContractions(text)
		scoreDialogue(text, candidates)

		// Build co-occurrence: collect all candidate names in this line.
		names := findLineNames(text, candidates)
		for i, a := range names {
			for _, b := range names[i+1:] {
				graph.add(a, b)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	return rank(candidates, graph), nil
}

// extractDialogue parses a single ASS line and returns the dialogue text.
// ASS format: Dialogue: layer,start,end,style,name,marginL,marginR,marginV,effect,text
func extractDialogue(line string) string {
	if !strings.HasPrefix(line, "Dialogue:") {
		return ""
	}

	// Find the 9th comma — text starts after it.
	rest := line[len("Dialogue:"):]
	var count int
	for i, r := range rest {
		if r == ',' {
			count++
			if count == 9 {
				text := rest[i+1:]
				return cleanDialogueText(text)
			}
		}
	}
	return ""
}

// cleanDialogueText strips ASS formatting from a dialogue string.
func cleanDialogueText(text string) string {
	// Replace ASS newline markers with spaces.
	text = strings.ReplaceAll(text, `\N`, " ")

	// Remove ASS formatting blocks { ... }.
	var b strings.Builder
	inBlock := false
	for _, r := range text {
		switch r {
		case '{':
			inBlock = true
		case '}':
			inBlock = false
		default:
			if !inBlock {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// scoreDialogue analyses a cleaned dialogue line and updates candidates.
func scoreDialogue(text string, candidates map[string]*Candidate) {
	// 0. Portmanteau decomposition — "MaiAji" → Mai + Ajisai.
	decomposePortmanteaux(text, candidates)

	// 1. Honouifics — strongest signal.
	for _, m := range honourificRx.FindAllStringSubmatch(text, -1) {
		name := normaliseName(m[1])
		c := candidates[name]
		if c == nil {
			c = &Candidate{Name: name, Honourific: true}
			candidates[name] = c
		} else {
			c.Honourific = true
		}
		c.Score += wHonorific
		c.Count++
	}

	// 2. Vocatives — strong signal.
	for _, m := range vocativeRx.FindAllStringSubmatch(text, -1) {
		base := stripHonourific(m[1])
		base = normaliseName(base)
		if base == "" || isEnglishWord(base) || hasEnglishSuffix(base) {
			continue
		}
		addScore(candidates, base, wVocative)
	}

	// 2b. Speech verbs — "Name said", "Name replied".
	for _, m := range speechVerbRx.FindAllStringSubmatch(text, -1) {
		base := stripHonourific(m[1])
		base = normaliseName(base)
		if base == "" || isEnglishWord(base) || hasEnglishSuffix(base) {
			continue
		}
		addScore(candidates, base, wSpeechVerb)
	}

	// 3. Mid-sentence proper nouns.
	sentences := sentenceRx.Split(text, -1)
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		words := strings.Fields(s)
		for i, raw := range words {
			// Skip first word of sentence — too many false positives.
			if i == 0 {
				continue
			}

			tok := cleanToken(raw)
			if tok == "" || len(tok) < minLen {
				continue
			}

			// Must start with uppercase letter.
			runes := []rune(tok)
			if !unicode.IsUpper(runes[0]) {
				continue
			}

			// Hard rejections.
			if isEnglishWord(tok) {
				continue
			}
			if hasEnglishSuffix(tok) {
				continue
			}

			// Convert to canonical form.
			name := normaliseName(tok)

			// Strip honourific suffixes so "Satuki-san" and "Satuki" merge.
			if base := stripHonourific(name); base != name {
				name = normaliseName(base)
			}

			score := wMidSentence

			// Bonus: inside dialogue quotes.
			if inQuotes(text, raw) {
				score += wInQuotes
			}

			// Bonus: longer names are more likely legitimate.
			if len(name) >= 5 {
				score += wLongName
			}

			// Penalty: ALL CAPS.
			if isAllUpper(raw) {
				score -= pCaps
				if score <= 0 {
					continue
				}
			}

			// Bonus: Japanese-sounding.
			if soundsJapanese(name) {
				score += wJapanese
			}

			if score > 0 {
				addScore(candidates, name, score)
			}
		}
	}

	// 4. Also score bare proper nouns (catches names that aren't mid-sentence
	// but appear as standalone tokens, e.g. "Amaori." as its own sentence).
	for _, m := range properRx.FindAllString(text, -1) {
		tok := cleanToken(m)
		if tok == "" || len(tok) < minLen {
			continue
		}
		if isEnglishWord(tok) || hasEnglishSuffix(tok) {
			continue
		}
		name := normaliseName(tok)
		// Check base name after stripping any honourific (e.g. "Satuki-san").
		base := stripHonourific(name)
		if base != name {
			base = normaliseName(base)
		}
		// Don't double-count if already scored via other pathways.
		if _, exists := candidates[name]; exists {
			continue
		}
		if _, exists := candidates[base]; exists {
			continue
		}
		score := 1 // minimal score for standalone appearance
		if len(name) >= 5 {
			score += wLongName
		}
		if soundsJapanese(name) {
			score += wJapanese
		}
		addScore(candidates, name, score)
	}

	// 5. ALL CAPS name recovery — e.g. "SENA:" overlay text, speaker labels.
	for _, m := range allCapsRx.FindAllString(text, -1) {
		tok := cleanToken(m)
		if tok == "" || len(tok) < 2 {
			continue
		}
		// Skip known English words and common fragments.
		if isEnglishWord(tok) {
			continue
		}
		if hasEnglishSuffix(tok) {
			continue
		}
		// Normalise to title case so "AJISAI" → "Ajisai" (merges with existing).
		runes := []rune(tok)
		titleTok := string(append([]rune{unicode.ToUpper(runes[0])}, []rune(strings.ToLower(string(runes[1:])))...))
		name := normaliseName(titleTok)
		if name == "" {
			continue
		}
		// Skip single-letter and very short tokens.
		if len(name) < 3 {
			continue
		}
		// Skip pure numbers.
		if strings.ContainsAny(name, "0123456789") {
			continue
		}
		// Check if followed by colon (speaker label pattern "SENA: ...").
		score := wAllCapsName
		// Find position of this match in the text to look for colon.
		idx := strings.Index(text, m)
		if idx >= 0 {
			remaining := text[idx+len(m):]
			if strings.HasPrefix(remaining, ":") {
				score += 2 // speaker label bonus
			}
		}
		addScore(candidates, name, score)
	}
}

// expandContractions replaces English contractions with their expanded forms.
// Handles both ASCII (') and Unicode (') apostrophes.
func expandContractions(s string) string {
	// Normalise Unicode apostrophes to ASCII.
	s = strings.ReplaceAll(s, "\u2019", "'")
	s = strings.ReplaceAll(s, "\u2018", "'")

	words := strings.Fields(s)
	var b strings.Builder
	for i, w := range words {
		if i > 0 {
			b.WriteByte(' ')
		}
		lower := strings.ToLower(w)
		if expanded, ok := contractions[lower]; ok {
			b.WriteString(expanded)
		} else {
			b.WriteString(w)
		}
	}
	return b.String()
}

// stripHonourific removes a Japanese honourific suffix from a name.
func stripHonourific(s string) string {
	before, _, ok := strings.Cut(s, "-")
	if !ok {
		return s
	}
	return before
}

// normaliseName cleans and aliases a token into canonical form.
func normaliseName(s string) string {
	s = cleanToken(s)
	if alias, ok := aliases[s]; ok {
		return alias
	}
	return s
}

// cleanToken strips surrounding punctuation and contraction fragments.
// Handles both ASCII (') and Unicode (') apostrophes.
func cleanToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `""''«»「」『』（）()[]{}<>.,!?:;`)
	// Handle apostrophes: keep only the part before the apostrophe.
	// Check ASCII apostrophe first.
	if idx := strings.IndexByte(s, '\''); idx >= 0 {
		s = s[:idx]
	} else if idx := strings.Index(s, "\u2019"); idx >= 0 {
		// Unicode apostrophe.
		s = s[:idx]
	}
	return s
}

// inQuotes reports whether word appears inside "..." or 「...».
func inQuotes(text, word string) bool {
	before, after, ok := strings.Cut(text, word)
	if !ok {
		return false
	}
	return strings.Contains(before, "\"") && strings.Contains(after, "\"")
}

// soundsJapanese returns true if the name has a high vowel density and ends
// with a vowel — both common in Japanese romanised names.
func soundsJapanese(s string) bool {
	if len(s) < 3 {
		return false
	}

	var vowels, consonants int
	var last rune
	for i, r := range s {
		switch r {
		case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
			vowels++
		default:
			if unicode.IsLetter(r) {
				consonants++
			}
		}
		if i == len(s)-1 {
			last = r
		}
	}

	if vowels+consonants == 0 {
		return false
	}

	endsVowel := false
	switch last {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		endsVowel = true
	}

	// Japanese has ~50% vowel density due to CV syllable structure.
	ratio := float64(vowels) / float64(vowels+consonants)
	return endsVowel && ratio > 0.35 && consonants >= 1
}

// isAllUpper reports whether all letters in s are uppercase.
func isAllUpper(s string) bool {
	return strings.ToUpper(s) == s && strings.ToLower(s) != s
}

// addScore increments a candidate's score and count.
func addScore(candidates map[string]*Candidate, name string, score int) {
	if name == "" {
		return
	}
	c, ok := candidates[name]
	if !ok {
		c = &Candidate{Name: name}
		candidates[name] = c
	}
	c.Score += score
	c.Count++
}

// findLineNames returns the canonical names of all candidates present in a
// dialogue line. Used to build the co-occurrence graph.
func findLineNames(text string, candidates map[string]*Candidate) []string {
	seen := make(map[string]bool)
	var result []string

	addIfCandidate := func(name string) {
		if _, ok := candidates[name]; ok && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}

	// Check all proper noun matches.
	for _, m := range properRx.FindAllString(text, -1) {
		tok := cleanToken(m)
		if tok == "" || len(tok) < minLen {
			continue
		}
		if isEnglishWord(tok) || hasEnglishSuffix(tok) {
			continue
		}
		name := normaliseName(tok)
		addIfCandidate(name)

		// Also check base after honourific strip (e.g. "Satuki-san" → "Satuki").
		if base := stripHonourific(name); base != name {
			addIfCandidate(normaliseName(base))
		}
	}

	// Also check explicit honourific matches.
	for _, m := range honourificRx.FindAllStringSubmatch(text, -1) {
		addIfCandidate(normaliseName(m[1]))
	}

	return result
}

// rank filters and sorts candidates by score descending, applying
// co-occurrence graph adjustments.
func rank(candidates map[string]*Candidate, g *coGraph) []Result {
	// Find max partners for connectedness normalization.
	maxPartners := 0
	for _, c := range candidates {
		if p := g.partners(c.Name); p > maxPartners {
			maxPartners = p
		}
	}

	var results []Result
	for _, c := range candidates {
		partners := g.partners(c.Name)
		score := c.Score

		// Apply graph-based adjustment.
		if partners == 0 && !c.Honourific {
			score -= 10 // isolation penalty (only for non-honourific names)
		} else if maxPartners > 0 {
			ratio := float64(partners) / float64(maxPartners)
			switch {
			case ratio > 0.6:
				score += 5 // strong connectedness bonus
			case ratio > 0.3:
				score += 2 // moderate connectedness bonus
			}
		}

		if score < minScore || c.Count < minCount {
			continue
		}
		results = append(results, Result{
			Name:  c.Name,
			Score: score,
			Count: c.Count,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Count > results[j].Count
		}
		return results[i].Score > results[j].Score
	})

	return results
}

// decomposePortmanteaux detects camelCase portmanteaux like "MaiAji" or
// "MaiSatu" and scores the decomposed name parts.
func decomposePortmanteaux(text string, candidates map[string]*Candidate) {
	// Match token with two name-like components: UppercaseLowercaseUppercaseLowercase
	pmRx := regexp.MustCompile(`[A-Z][a-z]+[A-Z][a-z]+`)
	for _, match := range pmRx.FindAllString(text, -1) {
		runes := []rune(match)
		// Try splitting at each uppercase letter (skip position 0).
		for i := 1; i < len(runes)-1; i++ {
			if !unicode.IsUpper(runes[i]) {
				continue
			}
			left := string(runes[:i])
			right := string(runes[i:])
			if len(left) < 2 || len(right) < 2 {
				continue
			}
			leftCanonical := normaliseName(left)
			rightCanonical := normaliseName(right)
			if leftCanonical == "" || rightCanonical == "" {
				continue
			}
			// Reject if either part is a known English word.
			if isEnglishWord(leftCanonical) || hasEnglishSuffix(leftCanonical) {
				continue
			}
			if isEnglishWord(rightCanonical) || hasEnglishSuffix(rightCanonical) {
				continue
			}
			// Score both parts if either is already a candidate or both look name-like.
			_, hasLeft := candidates[leftCanonical]
			_, hasRight := candidates[rightCanonical]
			if hasLeft && hasRight {
				addScore(candidates, leftCanonical, wPortmanteau)
				addScore(candidates, rightCanonical, wPortmanteau)
			} else if hasLeft && !isEnglishWord(rightCanonical) {
				addScore(candidates, leftCanonical, wPortmanteau)
				addScore(candidates, rightCanonical, wPortmanteau/2)
			} else if hasRight && !isEnglishWord(leftCanonical) {
				addScore(candidates, leftCanonical, wPortmanteau/2)
				addScore(candidates, rightCanonical, wPortmanteau)
			}
		}
	}
}
