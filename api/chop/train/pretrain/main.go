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

	fmt.Println("Loading dataset...")
	dataset, err := data.LoadNamesDataset()
	if err != nil {
		panic(err)
	}

	r.Shuffle(len(dataset.Docs), func(i, j int) {
		dataset.Docs[i], dataset.Docs[j] = dataset.Docs[j], dataset.Docs[i]
	})
	fmt.Printf("num docs: %d\n", len(dataset.Docs))
	fmt.Printf("vocab size: %d\n", dataset.Tokenizer.VocabSize)

	config := model.Config{
		NLayer:    1,
		NEmbd:     16,
		BlockSize: 16,
		NHead:     4,
		VocabSize: dataset.Tokenizer.VocabSize,
	}

	fmt.Println("Initializing GPT Model...")
	gpt := model.NewGPTModel(r, config)
	fmt.Printf("num params: %d\n", len(gpt.Params))

	learningRate := 0.01
	adam := trainer.NewAdamOptimizer(gpt.Params, learningRate)

	numSteps := 1000
	fmt.Println("Starting Pre-Training Loop...")
	startTime := time.Now()

	for step := 0; step < numSteps; step++ {
		doc := dataset.Docs[step%len(dataset.Docs)]
		tokens := dataset.Tokenizer.Encode(doc)

		n := config.BlockSize
		if len(tokens)-1 < n {
			n = len(tokens) - 1
		}

		keys := make([][][]*tensor.Value, config.NLayer)
		values := make([][][]*tensor.Value, config.NLayer)

		var losses []*tensor.Value
		for posId := 0; posId < n; posId++ {
			tokenId := tokens[posId]
			targetId := tokens[posId+1]

			logits := gpt.Forward(tokenId, posId, keys, values)
			probs := tensor.Softmax(logits)
			lossT := probs[targetId].Log().Neg()
			losses = append(losses, lossT)
		}

		sumLoss := tensor.NewValue(0, nil, nil)
		for _, l := range losses {
			sumLoss = sumLoss.Add(l)
		}
		loss := sumLoss.TrueDiv(tensor.NewValue(float64(n), nil, nil))

		loss.Backward()

		lrT := learningRate * (1.0 - float64(step)/float64(numSteps))
		adam.Step(lrT)

		fmt.Printf("step %4d / %4d | loss %.4f\r", step+1, numSteps, loss.Data)
	}
	fmt.Printf("\nTraining completed in %s.\n", time.Since(startTime))

	temperature := 0.5
	fmt.Println("\n--- Inference (new generated names) ---")
	for sampleIdx := 0; sampleIdx < 20; sampleIdx++ {
		keys := make([][][]*tensor.Value, config.NLayer)
		values := make([][][]*tensor.Value, config.NLayer)
		tokenId := dataset.Tokenizer.BOS
		var sample []rune

		for posId := 0; posId < config.BlockSize; posId++ {
			logits := gpt.Forward(tokenId, posId, keys, values)
			var scaledLogits []*tensor.Value
			for _, l := range logits {
				scaledLogits = append(scaledLogits, l.TrueDiv(tensor.NewValue(temperature, nil, nil)))
			}
			probs := tensor.Softmax(scaledLogits)

			weights := make([]float64, len(probs))
			for i, p := range probs {
				weights[i] = p.Data
			}

			tokenId = tensor.GetRandChoices(r, weights)
			if tokenId == dataset.Tokenizer.BOS {
				break
			}
			sample = append(sample, dataset.Tokenizer.UChars[tokenId])
		}
		fmt.Printf("sample %2d: %s\n", sampleIdx+1, string(sample))
	}
}
