package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/a-digi/coco-iam/chop/data"
	"github.com/a-digi/coco-iam/chop/model"
	"github.com/a-digi/coco-iam/chop/tensor"
	"github.com/a-digi/coco-iam/chop/trainer"
)

func main() {
	r := rand.New(rand.NewSource(42))

	fmt.Println("=== Phase 2: Supervised Fine Tuning (SFT) ===")
	fmt.Println("Loading Albanian Instruction Dataset...")

	dataset, err := data.LoadQADataset("data/albanian_qa.parquet")
	if err != nil {
		panic(err)
	}

	fmt.Printf("num QA pairs: %d\n", len(dataset.Pairs))
	fmt.Printf("SFT vocab size: %d\n", dataset.Tokenizer.VocabSize)

	config := model.Config{
		NLayer:    1,
		NEmbd:     16,
		BlockSize: 64, // Needs to be longer to hold prompt AND answer
		NHead:     4,
		VocabSize: dataset.Tokenizer.VocabSize,
	}

	fmt.Println("Initializing GPT Model for SFT...")
	gpt := model.NewGPTModel(r, config)

	learningRate := 0.01
	adam := trainer.NewAdamOptimizer(gpt.Params, learningRate)

	numSteps := 2000
	fmt.Print("Starting SFT Training Loop... (learning to answer questions)\n\n")
	startTime := time.Now()

	for step := 0; step < numSteps; step++ {
		// Sample a random Prompt/Answer pair
		pair := dataset.Pairs[step%len(dataset.Pairs)]

		// The prompt logic: User asks a question, Assistant responds
		// e.g. "Prompt: Translate Yes\nAnswer: Po<BOS>"
		promptText := pair.Prompt + "\nAnswer: "
		fullText := promptText + pair.Completion

		// Tokenize full text and insert BOS at the end
		promptTokens := dataset.Tokenizer.Encode(promptText)
		promptLen := len(promptTokens)

		fullTokens := append([]int{dataset.Tokenizer.BOS}, dataset.Tokenizer.Encode(fullText)...)
		fullTokens = append(fullTokens, dataset.Tokenizer.BOS)

		n := config.BlockSize
		if len(fullTokens)-1 < n {
			n = len(fullTokens) - 1
		}

		keys := make([][][]*tensor.Value, config.NLayer)
		values := make([][][]*tensor.Value, config.NLayer)

		var losses []*tensor.Value
		var numUnmaskedTokens float64

		for posId := 0; posId < n; posId++ {
			tokenId := fullTokens[posId]
			targetId := fullTokens[posId+1]

			logits := gpt.Forward(tokenId, posId, keys, values)
			probs := tensor.Softmax(logits)
			lossT := probs[targetId].Log().Neg()

			// *** LOSS MASKING LOGIC ***
			// We DO NOT penalize the model for failing to predict the User's Prompt.
			// The model is only penalized for tokens strictly inside the Answer.
			if posId < promptLen {
				// MASK OUT this token's loss by multiplying by 0
				lossT = lossT.Mul(tensor.NewValue(0, nil, nil))
			} else {
				numUnmaskedTokens++
			}

			losses = append(losses, lossT)
		}

		sumLoss := tensor.NewValue(0, nil, nil)
		for _, l := range losses {
			sumLoss = sumLoss.Add(l)
		}

		// Average the loss ONLY over the unmasked answer tokens
		if numUnmaskedTokens == 0 {
			numUnmaskedTokens = 1.0 // safety
		}
		loss := sumLoss.TrueDiv(tensor.NewValue(numUnmaskedTokens, nil, nil))

		loss.Backward()

		lrT := learningRate * (1.0 - float64(step)/float64(numSteps))
		adam.Step(lrT)

		if step%100 == 0 || step == numSteps-1 {
			fmt.Printf("step %4d / %4d | loss %.4f\n", step+1, numSteps, loss.Data)
		}
	}
	fmt.Printf("\nSFT Training completed in %s.\n", time.Since(startTime))

	fmt.Println("Saving model weights to albanian_sft.bin...")
	if err := gpt.SaveWeights("albanian_sft.bin"); err != nil {
		panic(err)
	}
	fmt.Println("Weights saved successfully!")
}
