package tensor

import "math/rand"

func Matrix(r *rand.Rand, nout, nin int, std float64) [][]*Value {
	mat := make([][]*Value, nout)
	for i := 0; i < nout; i++ {
		row := make([]*Value, nin)
		for j := 0; j < nin; j++ {
			row[j] = NewValue(r.NormFloat64()*std, nil, nil)
		}
		mat[i] = row
	}
	return mat
}

func Linear(x []*Value, w [][]*Value) []*Value {
	nout := len(w)
	out := make([]*Value, nout)
	for i := 0; i < nout; i++ {
		wi := w[i]
		sum := NewValue(0, nil, nil)
		for j := 0; j < len(wi); j++ {
			sum = sum.Add(wi[j].Mul(x[j]))
		}
		out[i] = sum
	}
	return out
}

func Softmax(logits []*Value) []*Value {
	maxVal := logits[0].Data
	for _, val := range logits {
		if val.Data > maxVal {
			maxVal = val.Data
		}
	}

	exps := make([]*Value, len(logits))
	total := NewValue(0, nil, nil)
	maxValNode := NewValue(maxVal, nil, nil)

	for i, val := range logits {
		exps[i] = val.Sub(maxValNode).Exp()
		total = total.Add(exps[i])
	}

	out := make([]*Value, len(logits))
	for i, e := range exps {
		out[i] = e.TrueDiv(total)
	}
	return out
}

func RMSNorm(x []*Value) []*Value {
	n := float64(len(x))
	ms := NewValue(0, nil, nil)
	for _, xi := range x {
		ms = ms.Add(xi.Mul(xi))
	}
	ms = ms.TrueDiv(NewValue(n, nil, nil))
	scale := ms.Add(NewValue(1e-5, nil, nil)).Pow(-0.5)

	out := make([]*Value, len(x))
	for i, xi := range x {
		out[i] = xi.Mul(scale)
	}
	return out
}

func GetRandChoices(r *rand.Rand, weights []float64) int {
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	val := r.Float64() * sum
	cum := 0.0
	for i, w := range weights {
		cum += w
		if val < cum {
			return i
		}
	}
	return len(weights) - 1
}
