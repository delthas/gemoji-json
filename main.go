package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	variationSelector16 = "\uFE0F"
	defaultEmojiTestURL = "https://unicode.org/Public/latest/emoji/emoji-test.txt"
)

var skinTones = []string{
	"\U0001F3FB",
	"\U0001F3FC",
	"\U0001F3FD",
	"\U0001F3FE",
	"\U0001F3FF",
}

var textGlyphs = map[string]bool{
	"\U0001F237": true,
	"\U0001F202": true,
	"\U0001F170": true,
	"\U0001F171": true,
	"\U0001F17E": true,
	"\u00A9":     true,
	"\u00AE":     true,
	"\u2122":     true,
	"\u3030":     true,
}

type EmojiEntry struct {
	Emoji          string   `json:"emoji,omitempty"`
	Description    string   `json:"description,omitempty"`
	Category       string   `json:"category,omitempty"`
	Aliases        []string `json:"aliases"`
	Tags           []string `json:"tags"`
	UnicodeVersion string   `json:"unicode_version,omitempty"`
	IOSVersion     string   `json:"ios_version,omitempty"`
	SkinTones      bool     `json:"skin_tones,omitempty"`
}

type parsedEmoji struct {
	sequences      []string
	description    string
	unicodeVersion string
	skinTones      bool
}

var transliterations = map[rune]string{
	'\u2019': "'",
	'\u201C': "\"",
	'\u201D': "\"",
	'\u00C5': "A",
	'\u00E3': "a",
	'\u00E7': "c",
	'\u00E9': "e",
	'\u00ED': "i",
	'\u00F1': "n",
	'\u00F4': "o",
	'\u00FC': "u",
	'\u00A9': "(c)",
	'\u00AE': "(r)",
}

var nonWordRe = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
var eVersionRe = regexp.MustCompile(`^E(\d+(?:\.\d+)?) `)

func main() {
	emojiTestPath := flag.String("emoji-test", "", "path to emoji-test.txt (default: fetch from unicode.org)")
	emojiJSONPath := flag.String("emoji-json", "emoji.json", "path to existing emoji.json")
	flag.Parse()

	oldEntries, err := loadEmojiJSON(*emojiJSONPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", *emojiJSONPath, err)
		os.Exit(1)
	}
	index := buildIndex(oldEntries)

	var testReader io.ReadCloser
	if *emojiTestPath != "" {
		f, err := os.Open(*emojiTestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", *emojiTestPath, err)
			os.Exit(1)
		}
		testReader = f
	} else {
		resp, err := http.Get(defaultEmojiTestURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching emoji-test.txt: %v\n", err)
			os.Exit(1)
		}
		if resp.StatusCode != 200 {
			fmt.Fprintf(os.Stderr, "Error fetching emoji-test.txt: HTTP %d\n", resp.StatusCode)
			os.Exit(1)
		}
		testReader = resp.Body
	}
	parsed, categories, err := parseEmojiTest(testReader)
	testReader.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing emoji-test.txt: %v\n", err)
		os.Exit(1)
	}

	items := merge(parsed, categories, oldEntries, index)

	w := bufio.NewWriter(os.Stdout)
	writeJSON(w, items)
	w.Flush()
}

func loadEmojiJSON(path string) ([]EmojiEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []EmojiEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func buildIndex(entries []EmojiEntry) map[string]*EmojiEntry {
	idx := make(map[string]*EmojiEntry)
	for i := range entries {
		e := &entries[i]
		raw := e.Emoji
		if raw == "" {
			continue
		}
		appendUnicode(idx, e, raw)

		startPos := 0
		found := false
		for {
			j := strings.Index(raw[startPos:], variationSelector16)
			if j < 0 {
				break
			}
			pos := startPos + j
			found = true
			alt := raw[:pos] + raw[pos+len(variationSelector16):]
			appendUnicode(idx, e, alt)
			startPos = pos + len(variationSelector16)
		}
		if found {
			appendUnicode(idx, e, strings.ReplaceAll(raw, variationSelector16, ""))
		} else {
			appendUnicode(idx, e, raw+variationSelector16)
		}
	}
	return idx
}

func appendUnicode(idx map[string]*EmojiEntry, e *EmojiEntry, raw string) {
	if textGlyphs[raw] {
		return
	}
	if _, exists := idx[raw]; !exists {
		idx[raw] = e
	}
}

func findByUnicode(idx map[string]*EmojiEntry, raw string) *EmojiEntry {
	if e, ok := idx[raw]; ok {
		return e
	}
	stripped := stripSkinTones(raw)
	if stripped != raw {
		if e, ok := idx[stripped]; ok {
			return e
		}
	}
	return nil
}

func stripSkinTones(s string) string {
	for _, tone := range skinTones {
		s = strings.ReplaceAll(s, tone, "")
	}
	return s
}

func hasSkinTone(s string) bool {
	for _, tone := range skinTones {
		if strings.Contains(s, tone) {
			return true
		}
	}
	return false
}

func normalize(s string) string {
	s = strings.ReplaceAll(s, variationSelector16, "")
	return stripSkinTones(s)
}

type parsedItem struct {
	emoji    *parsedEmoji
	category string
}

func parseEmojiTest(r io.Reader) ([]parsedItem, []string, error) {
	scanner := bufio.NewScanner(r)
	emojiMap := make(map[string]*parsedEmoji)
	var items []parsedItem
	var categories []string
	var currentCategory string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# group: ") {
			currentCategory = strings.TrimPrefix(line, "# group: ")
			categories = append(categories, currentCategory)
			continue
		}
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, "#", 2)
		if len(parts) < 2 {
			continue
		}
		rowParts := strings.SplitN(parts[0], ";", 2)
		if len(rowParts) < 2 {
			continue
		}
		qualification := strings.TrimSpace(rowParts[1])
		if qualification == "unqualified" || qualification == "component" {
			continue
		}

		codepoints := strings.TrimSpace(rowParts[0])
		emojiRaw := codepointsToString(codepoints)

		desc := strings.TrimSpace(parts[1])
		// desc looks like: "😀 E1.0 grinning face"
		// Split to skip the emoji char, keep "E1.0 grinning face"
		if idx := strings.IndexByte(desc, ' '); idx >= 0 {
			desc = desc[idx+1:]
		}

		normalized := normalize(emojiRaw)
		existing := emojiMap[normalized]

		if hasSkinTone(emojiRaw) {
			if existing != nil {
				existing.skinTones = true
			}
			continue
		}

		var unicodeVersion string
		if m := eVersionRe.FindStringSubmatch(desc); m != nil {
			unicodeVersion = m[1]
			desc = desc[len(m[0]):]
		}

		if existing != nil {
			existing.sequences = append(existing.sequences, emojiRaw)
		} else {
			pe := &parsedEmoji{
				sequences:      []string{emojiRaw},
				description:    desc,
				unicodeVersion: unicodeVersion,
			}
			emojiMap[normalized] = pe
			items = append(items, parsedItem{emoji: pe, category: currentCategory})
		}
	}
	return items, categories, scanner.Err()
}

func codepointsToString(s string) string {
	parts := strings.Fields(s)
	runes := make([]rune, len(parts))
	for i, p := range parts {
		n, _ := strconv.ParseInt(p, 16, 32)
		runes[i] = rune(n)
	}
	return string(runes)
}

func merge(parsed []parsedItem, _ []string, oldEntries []EmojiEntry, idx map[string]*EmojiEntry) []EmojiEntry {
	seen := make(map[*EmojiEntry]bool)
	var items []EmojiEntry

	for _, pi := range parsed {
		raw := pi.emoji.sequences[0]

		old := findByUnicode(idx, raw)
		if old == nil {
			old = findByUnicode(idx, raw+variationSelector16)
		}
		if old != nil && seen[old] {
			old = nil
		}
		if old != nil {
			seen[old] = true
		}

		entry := EmojiEntry{
			Emoji:       raw,
			Description: pi.emoji.description,
			Category:    pi.category,
		}
		if old != nil {
			entry.Aliases = old.Aliases
			entry.Tags = old.Tags
			entry.UnicodeVersion = old.UnicodeVersion
			entry.IOSVersion = old.IOSVersion
		} else {
			entry.Aliases = []string{generateAlias(pi.emoji.description)}
			entry.Tags = []string{}
			entry.UnicodeVersion = pi.emoji.unicodeVersion
		}
		if pi.emoji.skinTones {
			entry.SkinTones = true
		}
		items = append(items, entry)
	}

	// Check for unmatched old entries
	var missing []EmojiEntry
	for i := range oldEntries {
		e := &oldEntries[i]
		if e.Emoji != "" && !seen[e] {
			missing = append(missing, *e)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "Error: these emoji.json entries were not matched:\n")
		for _, e := range missing {
			fmt.Fprintf(os.Stderr, "%s (%s)\n", hexInspect(e.Emoji), e.Aliases[0])
		}
		os.Exit(1)
	}

	// Append custom emoji (no "emoji" field)
	for _, e := range oldEntries {
		if e.Emoji == "" {
			items = append(items, EmojiEntry{
				Aliases: e.Aliases,
				Tags:    e.Tags,
			})
		}
	}

	return items
}

func hexInspect(s string) string {
	var parts []string
	for _, r := range s {
		parts = append(parts, fmt.Sprintf("%04X", r))
	}
	return strings.Join(parts, " ")
}

func transliterate(s string) string {
	var buf strings.Builder
	for _, r := range s {
		if r < 128 {
			buf.WriteRune(r)
		} else if repl, ok := transliterations[r]; ok {
			buf.WriteString(repl)
		} else {
			buf.WriteRune('?')
		}
	}
	return buf.String()
}

func generateAlias(description string) string {
	s := transliterate(description)
	s = nonWordRe.ReplaceAllString(s, "_")
	s = strings.ToLower(s)
	return s
}

func jsonStr(s string) string {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

func writeJSON(w *bufio.Writer, items []EmojiEntry) {
	w.WriteString("[\n")
	for i, item := range items {
		if i == 0 {
			w.WriteString("  {\n")
		} else {
			w.WriteString(", {\n")
		}

		if item.Emoji != "" {
			fmt.Fprintf(w, "    \"emoji\": %s\n", jsonStr(item.Emoji))
			fmt.Fprintf(w, "  , \"description\": %s\n", jsonStr(item.Description))
			fmt.Fprintf(w, "  , \"category\": %s\n", jsonStr(item.Category))
		}

		w.WriteString("  , \"aliases\": [\n")
		writeStringArray(w, item.Aliases)

		w.WriteString("  , \"tags\": [\n")
		writeStringArray(w, item.Tags)

		if item.Emoji != "" {
			fmt.Fprintf(w, "  , \"unicode_version\": %s\n", jsonStr(item.UnicodeVersion))
			if item.IOSVersion != "" {
				fmt.Fprintf(w, "  , \"ios_version\": %s\n", jsonStr(item.IOSVersion))
			}
		}

		if item.SkinTones {
			w.WriteString("  , \"skin_tones\": true\n")
		}

		w.WriteString("  }\n")
	}
	w.WriteString("]\n")
}

func writeStringArray(w *bufio.Writer, arr []string) {
	if len(arr) == 0 {
		w.WriteString("    ]\n")
		return
	}
	for j, val := range arr {
		if j == 0 {
			fmt.Fprintf(w, "      %s\n", jsonStr(val))
		} else {
			fmt.Fprintf(w, "    , %s\n", jsonStr(val))
		}
	}
	w.WriteString("    ]\n")
}
