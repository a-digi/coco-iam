package data

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/parquet-go/parquet-go"
)

type Tokenizer struct {
	UChars    []rune
	VocabSize int
	BOS       int
}

type Dataset struct {
	Docs      []string
	Tokenizer *Tokenizer
}

type QAPair struct {
	Prompt     string `json:"prompt" parquet:"prompt"`
	Completion string `json:"completion" parquet:"completion"`
}

type QADataset struct {
	Pairs     []QAPair
	Tokenizer *Tokenizer
}

func LoadNamesDataset() (*Dataset, error) {
	if _, err := os.Stat("input.txt"); os.IsNotExist(err) {
		resp, err := http.Get("https://raw.githubusercontent.com/karpathy/makemore/988aa59/names.txt")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		out, err := os.Create("input.txt")
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(out, resp.Body)
		out.Close()
		if err != nil {
			return nil, err
		}
	}

	file, err := os.Open("input.txt")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var docs []string
	scanner := bufio.NewScanner(file)
	charSet := make(map[rune]bool)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			docs = append(docs, line)
			for _, ch := range line {
				charSet[ch] = true
			}
		}
	}

	var uchars []rune
	for ch := range charSet {
		uchars = append(uchars, ch)
	}
	sort.Slice(uchars, func(i, j int) bool {
		return uchars[i] < uchars[j]
	})

	BOS := len(uchars)
	vocabSize := len(uchars) + 1

	return &Dataset{
		Docs: docs,
		Tokenizer: &Tokenizer{
			UChars:    uchars,
			VocabSize: vocabSize,
			BOS:       BOS,
		},
	}, nil
}

func LoadQADataset(path string) (*QADataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := parquet.NewGenericReader[QAPair](file)
	defer reader.Close()

	var pairs []QAPair
	var batch = make([]QAPair, 100)
	for {
		n, err := reader.Read(batch)
		pairs = append(pairs, batch[:n]...)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}

	charSet := make(map[rune]bool)
	for _, pair := range pairs {
		for _, ch := range pair.Prompt {
			charSet[ch] = true
		}
		for _, ch := range pair.Completion {
			charSet[ch] = true
		}
	}

	charSet['\n'] = true

	var uchars []rune
	for ch := range charSet {
		uchars = append(uchars, ch)
	}
	// Also ensure we have a standard vocabulary for common things
	// E.g., numbers, alphabet
	sort.Slice(uchars, func(i, j int) bool {
		return uchars[i] < uchars[j]
	})

	BOS := len(uchars)
	vocabSize := len(uchars) + 1

	return &QADataset{
		Pairs: pairs,
		Tokenizer: &Tokenizer{
			UChars:    uchars,
			VocabSize: vocabSize,
			BOS:       BOS,
		},
	}, nil
}

func (t *Tokenizer) Encode(text string) []int {
	var tokens []int
	for _, ch := range text {
		for idx, u := range t.UChars {
			if u == ch {
				tokens = append(tokens, idx)
				break
			}
		}
	}
	return tokens
}

func (t *Tokenizer) Decode(tokens []int) string {
	var result string
	for _, tok := range tokens {
		if tok >= 0 && tok < len(t.UChars) {
			result += string(t.UChars[tok])
		}
	}
	return result
}
