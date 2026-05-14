package trainer

import (
	"math"

	"github.com/a-digi/coco-iam/chop/tensor"
)

type AdamOptimizer struct {
	Params       []*tensor.Value
	LearningRate float64
	Beta1        float64
	Beta2        float64
	EpsAdam      float64
	M            []float64
	V            []float64
	StepCount    int
}

func NewAdamOptimizer(params []*tensor.Value, lr float64) *AdamOptimizer {
	return &AdamOptimizer{
		Params:       params,
		LearningRate: lr,
		Beta1:        0.85,
		Beta2:        0.99,
		EpsAdam:      1e-8,
		M:            make([]float64, len(params)),
		V:            make([]float64, len(params)),
		StepCount:    0,
	}
}

func (adam *AdamOptimizer) Step(lrT float64) {
	adam.StepCount++
	for i, p := range adam.Params {
		adam.M[i] = adam.Beta1*adam.M[i] + (1-adam.Beta1)*p.Grad
		adam.V[i] = adam.Beta2*adam.V[i] + (1-adam.Beta2)*(p.Grad*p.Grad)
		mHat := adam.M[i] / (1 - math.Pow(adam.Beta1, float64(adam.StepCount)))
		vHat := adam.V[i] / (1 - math.Pow(adam.Beta2, float64(adam.StepCount)))
		p.Data -= lrT * mHat / (math.Pow(vHat, 0.5) + adam.EpsAdam)
		p.Grad = 0
	}
}
