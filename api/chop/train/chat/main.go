package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/a-digi/coco-iam/chop/data"
	"github.com/a-digi/coco-iam/chop/model"
	"github.com/a-digi/coco-iam/chop/tensor"
)

func main() {
	r := rand.New(rand.NewSource(42))

	fmt.Println("Loading Albanian Instruction Dataset for Vocabulary...")
	dataset, err := data.LoadQADataset("data/albanian_qa.parquet")
	if err != nil {
		panic(err)
	}

	config := model.Config{
		NLayer:    1,
		NEmbd:     16,
		BlockSize: 64,
		NHead:     4,
		VocabSize: dataset.Tokenizer.VocabSize,
	}

	gpt := model.NewGPTModel(r, config)

	fmt.Println("Loading trained model weights from albanian_sft.bin...")
	if err := gpt.LoadWeights("albanian_sft.bin"); err != nil {
		fmt.Println("Could not load weights, please run the supervised_fine_tuning script first!")
		panic(err)
	}
	fmt.Println("Weights loaded successfully!")

	fmt.Println("\n==============================================")
	fmt.Println("      Albanian Translator Assistant (Toy)     ")
	fmt.Println("==============================================")
	fmt.Println("Ask a question! Type 'exit' to quit.")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\nUser: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "exit" || input == "quit" {
			break
		}

		// Interactive Inference Logic:
		// 1. Format the interaction Context Prompt
		promptText := fmt.Sprintf("Prompt: %s\nAnswer: ", input)
		tokens := append([]int{dataset.Tokenizer.BOS}, dataset.Tokenizer.Encode(promptText)...)

		keys := make([][][]*tensor.Value, config.NLayer)
		values := make([][][]*tensor.Value, config.NLayer)

		// 2. Feed the prompt into the GPT to build the context state (kv cache)
		var lastTokenId int
		for posId := 0; posId < len(tokens); posId++ {
			lastTokenId = tokens[posId]
			// We discard logits here, we only care about building `keys` and `values` state
			gpt.Forward(lastTokenId, posId, keys, values)
		}

		// 3. Autoregressively generate the Assistant's answer
		fmt.Print("Assistant: ")
		for posId := len(tokens); posId < config.BlockSize; posId++ {
			logits := gpt.Forward(lastTokenId, posId, keys, values)

			// Use temperature near 0 (Greedy Decoding) to prevent random babbling
			var scaledLogits []*tensor.Value
			for _, l := range logits {
				scaledLogits = append(scaledLogits, l.TrueDiv(tensor.NewValue(0.1, nil, nil)))
			}
			probs := tensor.Softmax(scaledLogits)

			weights := make([]float64, len(probs))
			for i, p := range probs {
				weights[i] = p.Data
			}

			lastTokenId = tensor.GetRandChoices(r, weights)
			if lastTokenId == dataset.Tokenizer.BOS {
				break
			}
			fmt.Print(string(dataset.Tokenizer.UChars[lastTokenId]))
		}
		fmt.Println()
	}
}
