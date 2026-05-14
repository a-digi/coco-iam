# Go GPT Pipeline

This is a complete, dependency-free pure Go educational implementation of an autoregressive transformer, broken down into a modern Machine Learning pipeline architecture. Starting from Andrej Karpathy's `makemore` script, this repository lays out the structures for pre-training, fine-tuning, and alignment.

## Architecture

Modern LLM development relies on separate stages of model building. This module splits the codebase into reusable library logic (`pkg/`) and execution targets (`cmd/`).

### 1. Library Modules

- **`tensor/`**: Contains the Autograd engine (`value.go`) tracking computation graphs for reverse-mode automatic differentiation. It also provides ML matrix operations (`math.go`) such as Linear layers, RMSNorm, Multi-Head Attention, and MLP sequences.
- **`model/`**: Defines the `GPTModel` architecture, building position embeddings, token embeddings, and the main forward pass topology.
- **`data/`**: Manages the Tokenizer (translating text strings into integer arrays) and coordinates fetching/processing datasets.
- **`trainer/`**: Isolates optimizer logic such as `AdamOptimizer` managing learning rates and momentum buffers (`m`, `v`).

### 2. Training Execution (`cmd/`)

The pipeline represents the life-cycle of a modern assistant model.

- **`train/pretrain`**: Phase 1. The model ingests a massive dataset (unsupervised) and simply learns to predict the next token autoregressively over thousands of iterations.
  *Usage*: `go run ./train/pretrain`
  
- **`train/supervised_fine_tuning`**: Phase 2 (Supervised Fine-Tuning). The base model from Phase 1 is taught question/answer formatting by masking out prompt tokens so gradients are only derived from the model's generated answers.

- **`train/align`**: Phase 3 (RLHF / DPO). The SFT model learns safety and preferences by observing trios of Prompts, Chosen responses, and Rejected responses. Contrastive loss pushes the weights to increase the likelihood of the chosen text and decrease the likelihood of rejected text.

## Usage

To automatically fetch the names dataset and run the pre-training loop over 1000 steps with inference at the end:

```sh
cd api/chop
go run ./train/pretrain
```
