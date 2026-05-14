package model

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"

	"github.com/a-digi/coco-iam/chop/tensor"
)

type Config struct {
	NLayer    int
	NEmbd     int
	BlockSize int
	NHead     int
	VocabSize int
}

type GPTModel struct {
	Config    Config
	StateDict map[string][][]*tensor.Value
	Params    []*tensor.Value
}

func NewGPTModel(r *rand.Rand, cfg Config) *GPTModel {
	stateDict := make(map[string][][]*tensor.Value)
	stateDict["wte"] = tensor.Matrix(r, cfg.VocabSize, cfg.NEmbd, 0.08)
	stateDict["wpe"] = tensor.Matrix(r, cfg.BlockSize, cfg.NEmbd, 0.08)
	stateDict["lm_head"] = tensor.Matrix(r, cfg.VocabSize, cfg.NEmbd, 0.08)

	for i := 0; i < cfg.NLayer; i++ {
		stateDict[fmt.Sprintf("layer%d.attn_wq", i)] = tensor.Matrix(r, cfg.NEmbd, cfg.NEmbd, 0.08)
		stateDict[fmt.Sprintf("layer%d.attn_wk", i)] = tensor.Matrix(r, cfg.NEmbd, cfg.NEmbd, 0.08)
		stateDict[fmt.Sprintf("layer%d.attn_wv", i)] = tensor.Matrix(r, cfg.NEmbd, cfg.NEmbd, 0.08)
		stateDict[fmt.Sprintf("layer%d.attn_wo", i)] = tensor.Matrix(r, cfg.NEmbd, cfg.NEmbd, 0.08)
		stateDict[fmt.Sprintf("layer%d.mlp_fc1", i)] = tensor.Matrix(r, 4*cfg.NEmbd, cfg.NEmbd, 0.08)
		stateDict[fmt.Sprintf("layer%d.mlp_fc2", i)] = tensor.Matrix(r, cfg.NEmbd, 4*cfg.NEmbd, 0.08)
	}

	var keys = []string{"wte", "wpe", "lm_head"}
	for i := 0; i < cfg.NLayer; i++ {
		keys = append(keys, fmt.Sprintf("layer%d.attn_wq", i))
		keys = append(keys, fmt.Sprintf("layer%d.attn_wk", i))
		keys = append(keys, fmt.Sprintf("layer%d.attn_wv", i))
		keys = append(keys, fmt.Sprintf("layer%d.attn_wo", i))
		keys = append(keys, fmt.Sprintf("layer%d.mlp_fc1", i))
		keys = append(keys, fmt.Sprintf("layer%d.mlp_fc2", i))
	}

	var params []*tensor.Value
	for _, k := range keys {
		mat := stateDict[k]
		for _, row := range mat {
			params = append(params, row...)
		}
	}

	return &GPTModel{
		Config:    cfg,
		StateDict: stateDict,
		Params:    params,
	}
}

func (m *GPTModel) Forward(tokenId, posId int, keys, values [][][]*tensor.Value) []*tensor.Value {
	tokEmb := m.StateDict["wte"][tokenId]
	posEmb := m.StateDict["wpe"][posId]

	x := make([]*tensor.Value, len(tokEmb))
	for i := range tokEmb {
		x[i] = tokEmb[i].Add(posEmb[i])
	}
	x = tensor.RMSNorm(x)

	headDim := m.Config.NEmbd / m.Config.NHead

	for li := 0; li < m.Config.NLayer; li++ {
		// 1) Multi-head Attention block
		xResidual := make([]*tensor.Value, len(x))
		copy(xResidual, x)

		x = tensor.RMSNorm(x)
		q := tensor.Linear(x, m.StateDict[fmt.Sprintf("layer%d.attn_wq", li)])
		k := tensor.Linear(x, m.StateDict[fmt.Sprintf("layer%d.attn_wk", li)])
		v := tensor.Linear(x, m.StateDict[fmt.Sprintf("layer%d.attn_wv", li)])

		keys[li] = append(keys[li], k)
		values[li] = append(values[li], v)

		var xAttn []*tensor.Value
		for h := 0; h < m.Config.NHead; h++ {
			hs := h * headDim
			qH := q[hs : hs+headDim]
			kH := make([][]*tensor.Value, len(keys[li]))
			for t, ki := range keys[li] {
				kH[t] = ki[hs : hs+headDim]
			}
			vH := make([][]*tensor.Value, len(values[li]))
			for t, vi := range values[li] {
				vH[t] = vi[hs : hs+headDim]
			}

			attnLogits := make([]*tensor.Value, len(kH))
			scaleFactor := math.Pow(float64(headDim), 0.5)
			scaleVal := tensor.NewValue(scaleFactor, nil, nil)

			for t := 0; t < len(kH); t++ {
				sumProd := tensor.NewValue(0, nil, nil)
				for j := 0; j < headDim; j++ {
					sumProd = sumProd.Add(qH[j].Mul(kH[t][j]))
				}
				attnLogits[t] = sumProd.TrueDiv(scaleVal)
			}

			attnWeights := tensor.Softmax(attnLogits)
			headOut := make([]*tensor.Value, headDim)
			for j := 0; j < headDim; j++ {
				sumV := tensor.NewValue(0, nil, nil)
				for t := 0; t < len(vH); t++ {
					sumV = sumV.Add(attnWeights[t].Mul(vH[t][j]))
				}
				headOut[j] = sumV
			}
			xAttn = append(xAttn, headOut...)
		}

		x = tensor.Linear(xAttn, m.StateDict[fmt.Sprintf("layer%d.attn_wo", li)])
		for j := range x {
			x[j] = x[j].Add(xResidual[j])
		}

		// 2) MLP block
		xResidual = make([]*tensor.Value, len(x))
		copy(xResidual, x)
		x = tensor.RMSNorm(x)
		x = tensor.Linear(x, m.StateDict[fmt.Sprintf("layer%d.mlp_fc1", li)])
		for j := range x {
			x[j] = x[j].Relu()
		}
		x = tensor.Linear(x, m.StateDict[fmt.Sprintf("layer%d.mlp_fc2", li)])
		for j := range x {
			x[j] = x[j].Add(xResidual[j])
		}
	}
	return tensor.Linear(x, m.StateDict["lm_head"])
}

func (m *GPTModel) SaveWeights(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, p := range m.Params {
		if err := binary.Write(file, binary.LittleEndian, p.Data); err != nil {
			return err
		}
	}
	return nil
}

func (m *GPTModel) LoadWeights(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, p := range m.Params {
		if err := binary.Read(file, binary.LittleEndian, &p.Data); err != nil {
			return err
		}
	}
	return nil
}
